package tasks

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/parser"
)

const MaxImageBytes = 10 << 20

var ErrInvalidImage = errors.New("请选择有效的 JPEG/PNG 图片（不超过 10 MiB、4000 万像素）")
var imageKeyPattern = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png)$`)

type ImageInput struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	MIME string `json:"mime"`
	Size int    `json:"size"`
}

type ImageStore struct{ dir string }

func NewImageStore(imageDir string) *ImageStore {
	return &ImageStore{dir: filepath.Join(imageDir, "task-inputs")}
}

func validImage(data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxImageBytes {
		return "", ErrInvalidImage
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
		return "", ErrInvalidImage
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return "", ErrInvalidImage
	}
	if format == "jpeg" {
		return "image/jpeg", nil
	}
	return "image/png", nil
}

func (s *ImageStore) save(name string, data []byte) (*ImageInput, error) {
	mime, err := validImage(data)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return nil, fmt.Errorf("创建照片存储目录失败: %w", err)
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	ext := ".jpg"
	if mime == "image/png" {
		ext = ".png"
	}
	key := hex.EncodeToString(token[:]) + ext
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if len(name) > 200 {
		name = "photo" + ext
	}
	if err := os.WriteFile(filepath.Join(s.dir, key), data, 0600); err != nil {
		return nil, fmt.Errorf("保存照片失败: %w", err)
	}
	return &ImageInput{Key: key, Name: name, MIME: mime, Size: len(data)}, nil
}
func (s *ImageStore) path(input *ImageInput) (string, error) {
	if s == nil || input == nil || !imageKeyPattern.MatchString(input.Key) {
		return "", errors.New("无效的任务照片引用")
	}
	path := filepath.Join(s.dir, input.Key)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("任务照片已丢失，请重新上传")
	}
	if info.Size() <= 0 || info.Size() > MaxImageBytes {
		return "", ErrInvalidImage
	}
	return path, nil
}
func (s *ImageStore) read(input *ImageInput) ([]byte, string, error) {
	path, err := s.path(input)
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxImageBytes+1))
	if err != nil {
		return nil, "", err
	}
	mime, err := validImage(data)
	return data, mime, err
}

// WithImages 为队列注入共享视觉模型与持久化照片目录。
func WithImages(p *parser.ImageParser, store *ImageStore) func(*Service) {
	return func(s *Service) { s.imageParser = p; s.images = store }
}

func (s *Service) SubmitImage(req Request, name string, data []byte) (*models.Task, error) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	req.Type = ImageParse
	req.Input = Input{UseLLM: true}
	if req.Source != "components" && req.Source != "pre-stocks" {
		return nil, errors.New("图片解析需要有效来源页面")
	}
	if err := s.validateSource(req); err != nil {
		return nil, err
	}
	if s.imageParser == nil || !s.imageParser.Configured() {
		return nil, errors.New("LLM 未配置，请设置 LLM_BASE_URL、LLM_API_KEY 和 LLM_MODEL，并选择支持图片输入的模型")
	}
	if s.images == nil {
		return nil, errors.New("照片存储未配置")
	}
	input, err := s.images.save(name, data)
	if err != nil {
		return nil, err
	}
	req.Input.Image = input
	task, err := s.submit(req, nil)
	if err != nil {
		_ = os.Remove(filepath.Join(s.images.dir, input.Key))
	}
	return task, err
}

func (s *Service) ImagePath(task *models.Task) (string, error) {
	if task.Type != ImageParse {
		return "", errors.New("此任务没有照片")
	}
	var input Input
	if err := json.Unmarshal(task.Input, &input); err != nil {
		return "", err
	}
	return s.images.path(input.Image)
}
