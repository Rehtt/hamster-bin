// Package tasks 实现单服务实例的持久化数据库任务队列。
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"gorm.io/gorm"
)

const ComponentParse = "component_parse"
const QRCodeParse = "qrcode_parse"
const ImageParse = "image_parse"

func ValidType(kind string) bool {
	return kind == ComponentParse || kind == QRCodeParse || kind == ImageParse
}

type Input struct {
	Code       string      `json:"code,omitempty"`
	QRCodeData string      `json:"qrcode_data,omitempty"`
	UseLLM     bool        `json:"use_llm"`
	Image      *ImageInput `json:"image,omitempty"`
}
type Request struct {
	Type     string `json:"type"`
	Input    Input  `json:"input"`
	Source   string `json:"source"`
	RecordID *uint  `json:"record_id,omitempty"`
}

func (s *Service) ImageRequest(source string, recordID *uint) Request {
	return Request{Type: ImageParse, Source: source, RecordID: recordID}
}

type Result struct {
	Component  *parser.ComponentInfo `json:"component,omitempty"`
	Items      []parser.ImageItem    `json:"items,omitempty"`
	Quantity   int                   `json:"quantity,omitempty"`
	QRCodeInfo *parser.QRCodeData    `json:"qrcode_info,omitempty"`
}

// Executor 可替换为本地测试解析器；生产中只调用共享 ParserManager。
type Executor func(context.Context, string, Input) (*Result, error)
type Service struct {
	Repo        *repository.TaskRepository
	manager     *parser.ParserManager
	imageParser *parser.ImageParser
	images      *ImageStore
	execute     Executor
	workers     int
	timeout     time.Duration
	mu          sync.Mutex // 领取/注册上下文与取消串行化，消除取消早于注册的竞态。
	running     map[uint]context.CancelFunc
	ctx         context.Context
	stop        context.CancelFunc
	wg          sync.WaitGroup
}

func New(repo *repository.TaskRepository, manager *parser.ParserManager, workers int, timeout time.Duration, options ...func(*Service)) *Service {
	if workers < 1 {
		workers = 2
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	s := &Service{Repo: repo, manager: manager, workers: workers, timeout: timeout, running: make(map[uint]context.CancelFunc)}
	for _, option := range options {
		option(s)
	}
	s.execute = s.parse
	return s
}
func (s *Service) validateSource(req Request) error {
	if req.Source != "" && req.Source != "components" && req.Source != "pre-stocks" {
		return errors.New("无效的来源页面")
	}
	if req.RecordID != nil && (*req.RecordID == 0 || req.Source == "") {
		return errors.New("编辑记录需要有效来源和 ID")
	}
	return nil
}
func (s *Service) Validate(req Request) error {
	if !ValidType(req.Type) {
		return errors.New("不支持的任务类型")
	}
	if err := s.validateSource(req); err != nil {
		return err
	}
	if req.Type == ImageParse {
		if s.imageParser == nil || !s.imageParser.Configured() {
			return errors.New("LLM 未配置或未启用图片解析")
		}
		_, err := s.images.path(req.Input.Image)
		return err
	}
	if req.Input.Image != nil {
		return errors.New("编码任务不能包含照片")
	}
	if len(req.Input.Code) > 4096 || len(req.Input.QRCodeData) > 65536 {
		return errors.New("编码过长")
	}
	code := req.Input.Code
	if req.Type == QRCodeParse {
		qr, err := parser.ParseQRCode(req.Input.QRCodeData)
		if err != nil {
			return err
		}
		code = qr.Code
	}
	if code == "" || len(code) > 4096 || len(req.Input.QRCodeData) > 65536 {
		return errors.New("编码为空或过长")
	}
	return s.manager.Validate(code, parser.ParseOptions{UseLLM: req.Input.UseLLM})
}
func (s *Service) Submit(req Request, retryOf *uint) (*models.Task, error) {
	if err := s.Validate(req); err != nil {
		return nil, err
	}
	input, _ := json.Marshal(req.Input)
	t := &models.Task{Type: req.Type, Input: models.JSONText(input), Source: req.Source, RecordID: req.RecordID, Status: repository.TaskQueued, RetryOfID: retryOf}
	if err := s.Repo.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}
func (s *Service) Retry(id uint) (*models.Task, error) {
	t, err := s.Repo.Get(id)
	if err != nil {
		return nil, err
	}
	if t.Status != repository.TaskFailed && t.Status != repository.TaskCancelled {
		return nil, repository.ErrTaskState
	}
	var input Input
	if err := json.Unmarshal(t.Input, &input); err != nil {
		return nil, err
	}
	return s.Submit(Request{Type: t.Type, Input: input, Source: t.Source, RecordID: t.RecordID}, &id)
}
func (s *Service) Cancel(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Repo.Cancel(id); err != nil {
		return err
	}
	if cancel := s.running[id]; cancel != nil {
		cancel()
	}
	return nil
}
func (s *Service) Start() error {
	if err := s.Repo.Recover(); err != nil {
		return err
	}
	s.ctx, s.stop = context.WithCancel(context.Background())
	for range s.workers {
		s.wg.Add(1)
		go s.worker()
	}
	return nil
}
func (s *Service) Stop() {
	if s.stop != nil {
		s.mu.Lock()
		s.stop()
		s.mu.Unlock()
		s.wg.Wait()
	}
}
func (s *Service) worker() {
	defer s.wg.Done()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		t, err := s.Repo.Claim()
		if err == nil {
			ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
			s.running[t.ID] = cancel
			s.mu.Unlock()
			s.run(ctx, t)
			cancel()
			s.mu.Lock()
			delete(s.running, t.ID)
			s.mu.Unlock()
			continue
		}
		s.mu.Unlock()
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("领取任务失败: %v", err)
		}
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Service) invoke(ctx context.Context, t *models.Task) (result *Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("任务 %d panic: %v", t.ID, r)
			err = errors.New("任务执行异常，请重试")
		}
	}()
	var input Input
	if err = json.Unmarshal(t.Input, &input); err != nil {
		return nil, err
	}
	return s.execute(ctx, t.Type, input)
}
func (s *Service) run(ctx context.Context, t *models.Task) {
	result, err := s.invoke(ctx, t)
	status := repository.TaskSucceeded
	message, code := "", ""
	var data models.JSONText
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		status = repository.TaskFailed
		message = err.Error()
		code = "execution"
		switch {
		case s.ctx.Err() != nil:
			message = "服务中断，请重试"
			code = "interrupted"
		case errors.Is(err, context.DeadlineExceeded):
			message = "任务执行超时，请重试"
			code = "timeout"
		case errors.Is(err, parser.ErrImageContent):
			code = "content"
		case errors.Is(err, parser.ErrParserUpstream):
			message = "上游平台请求失败，请稍后重试"
			code = "upstream"
		case errors.Is(err, parser.ErrParserContent):
			message = "页面内容解析失败"
			code = "content"
		}
	} else {
		raw, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			status = repository.TaskFailed
			message = "任务结果编码失败"
			code = "execution"
		} else {
			data = models.JSONText(raw)
		}
	}
	if err := s.Repo.Finish(t.ID, status, data, message, code); err != nil {
		log.Printf("保存任务 %d 结果失败: %v", t.ID, err)
	}
}
func (s *Service) parse(ctx context.Context, kind string, input Input) (*Result, error) {
	if kind == ImageParse {
		data, mime, err := s.images.read(input.Image)
		if err != nil {
			return nil, err
		}
		items, err := s.imageParser.Parse(ctx, data, mime)
		if err != nil {
			return nil, err
		}
		return &Result{Items: items}, nil
	}
	result := &Result{}
	code := input.Code
	if kind == QRCodeParse {
		qr, err := parser.ParseQRCode(input.QRCodeData)
		if err != nil {
			return nil, err
		}
		result.QRCodeInfo = qr
		result.Quantity = qr.Quantity
		code = qr.Code
	}
	info, err := s.manager.ParseWithOptions(code, parser.ParseOptions{UseLLM: input.UseLLM, Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("解析失败: %w", err)
	}
	result.Component = info
	return result, nil
}
