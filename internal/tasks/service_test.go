package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testService(t *testing.T, workers int, timeout time.Duration) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tasks.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Task{}); err != nil {
		t.Fatal(err)
	}
	manager := parser.NewParserManager()
	manager.Register(parser.NewTaobaoParser())
	manager.Register(parser.NewLCSCParser())
	s := New(repository.NewTaskRepository(db), manager, workers, timeout)
	t.Cleanup(s.Stop)
	return s
}
func request() Request {
	return Request{Type: ComponentParse, Input: Input{Code: "123456789012"}, Source: "components"}
}
func submit(t *testing.T, s *Service) *models.Task {
	t.Helper()
	task, err := s.Submit(request(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return task
}
func waitTask(t *testing.T, s *Service, id uint, status string) *models.Task {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		task, err := s.Repo.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if task.Status == status {
			return task
		}
		time.Sleep(5 * time.Millisecond)
	}
	task, _ := s.Repo.Get(id)
	t.Fatalf("task %d expected %s, got %+v", id, status, task)
	return nil
}
func start(t *testing.T, s *Service) {
	t.Helper()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
}
func TestQueuePersistenceConcurrencyAndClaimOnce(t *testing.T) {
	s := testService(t, 2, time.Second)
	var active, max atomic.Int32
	var mu sync.Mutex
	calls := map[string]int{}
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	s.execute = func(ctx context.Context, kind string, input Input) (*Result, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for {
			old := max.Load()
			if n <= old || max.CompareAndSwap(old, n) {
				break
			}
		}
		mu.Lock()
		calls[input.Code]++
		mu.Unlock()
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &Result{Component: &parser.ComponentInfo{Name: input.Code}}, nil
	}
	ids := []uint{}
	for i := range 6 {
		req := request()
		req.Input.Code = string(rune('1'+i)) + "23456789012"
		task, err := s.Submit(req, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, task.ID)
	}
	// 提交仅落库，未启动 worker 时也可查询；新 repository 读取相同记录。
	found, err := s.Repo.Get(ids[0])
	if err != nil || found.Status != repository.TaskQueued {
		t.Fatalf("persistent task: %+v %v", found, err)
	}
	start(t, s)
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	select {
	case <-started:
		t.Fatal("exceeded concurrency")
	case <-time.After(40 * time.Millisecond):
	}
	close(release)
	for _, id := range ids {
		waitTask(t, s, id, repository.TaskSucceeded)
	}
	s.Stop()
	if max.Load() != 2 {
		t.Fatalf("max concurrency=%d", max.Load())
	}
	for code, count := range calls {
		if count != 1 {
			t.Fatalf("%s executed %d times", code, count)
		}
	}
}
func TestTimeoutCancellationAndRetry(t *testing.T) {
	s := testService(t, 1, 60*time.Millisecond)
	s.execute = func(ctx context.Context, _ string, _ Input) (*Result, error) {
		<-ctx.Done()
		return &Result{Component: &parser.ComponentInfo{Name: "late"}}, nil
	}
	timed := submit(t, s)
	start(t, s)
	failed := waitTask(t, s, timed.ID, repository.TaskFailed)
	if failed.ErrorCode != "timeout" || failed.StartedAt == nil || failed.FinishedAt == nil {
		t.Fatalf("timeout result: %+v", failed)
	}
	retried, err := s.Retry(timed.ID)
	if err != nil || retried.RetryOfID == nil || *retried.RetryOfID != timed.ID || retried.ID == timed.ID {
		t.Fatalf("retry: %+v %v", retried, err)
	}
	waitTask(t, s, retried.ID, repository.TaskRunning)
	if err := s.Cancel(retried.ID); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	cancelled := waitTask(t, s, retried.ID, repository.TaskCancelled)
	if cancelled.Result != nil {
		t.Fatal("late success overwrote cancellation")
	}
	if err := s.Cancel(retried.ID); !errors.Is(err, repository.ErrTaskState) {
		t.Fatalf("cancel terminal: %v", err)
	}
	original, _ := s.Repo.Get(timed.ID)
	if original.Status != repository.TaskFailed {
		t.Fatal("retry changed original")
	}
}
func TestQueuedCancelPanicAndRestart(t *testing.T) {
	s := testService(t, 1, time.Second)
	cancelled := submit(t, s)
	if err := s.Cancel(cancelled.ID); err != nil {
		t.Fatal(err)
	}
	abandoned := submit(t, s)
	claimed, err := s.Repo.Claim()
	if err != nil || claimed.ID != abandoned.ID {
		t.Fatalf("claim oldest queued: %+v %v", claimed, err)
	}
	panicked := submit(t, s)
	next := submit(t, s)
	var calls atomic.Int32
	s.execute = func(context.Context, string, Input) (*Result, error) {
		if calls.Add(1) == 1 {
			panic("test panic")
		}
		return &Result{Component: &parser.ComponentInfo{Name: "ok"}}, nil
	}
	start(t, s)
	interrupted := waitTask(t, s, abandoned.ID, repository.TaskFailed)
	if interrupted.ErrorCode != "interrupted" {
		t.Fatalf("restart: %+v", interrupted)
	}
	waitTask(t, s, panicked.ID, repository.TaskFailed)
	waitTask(t, s, next.ID, repository.TaskSucceeded)
	if _, err := s.Retry(next.ID); !errors.Is(err, repository.ErrTaskState) {
		t.Fatalf("retry success: %v", err)
	}
	waitTask(t, s, cancelled.ID, repository.TaskCancelled)
}
func TestShutdownPreservesQueueAndRecordsInterruption(t *testing.T) {
	s := testService(t, 1, time.Second)
	s.execute = func(ctx context.Context, _ string, _ Input) (*Result, error) { <-ctx.Done(); return nil, ctx.Err() }
	running := submit(t, s)
	queued := submit(t, s)
	start(t, s)
	waitTask(t, s, running.ID, repository.TaskRunning)
	s.Stop()
	interrupted := waitTask(t, s, running.ID, repository.TaskFailed)
	if interrupted.ErrorCode != "interrupted" {
		t.Fatalf("shutdown: %+v", interrupted)
	}
	waitTask(t, s, queued.ID, repository.TaskQueued)
	s.execute = func(context.Context, string, Input) (*Result, error) {
		return &Result{Component: &parser.ComponentInfo{Name: "restarted"}}, nil
	}
	start(t, s)
	waitTask(t, s, queued.ID, repository.TaskSucceeded)
}
func TestCancelCompletionRace(t *testing.T) {
	s := testService(t, 2, time.Second)
	s.execute = func(context.Context, string, Input) (*Result, error) {
		return &Result{Component: &parser.ComponentInfo{Name: "done"}}, nil
	}
	start(t, s)
	for range 30 {
		task := submit(t, s)
		err := s.Cancel(task.ID)
		if err != nil && !errors.Is(err, repository.ErrTaskState) {
			t.Fatal(err)
		}
		current, _ := s.Repo.Get(task.ID)
		if err == nil && current.Status != repository.TaskCancelled {
			t.Fatal("cancel lost")
		}
		if err != nil && current.Status != repository.TaskSucceeded {
			t.Fatalf("unexpected terminal state %+v", current)
		}
	}
}
func TestValidationAndQRCodeResult(t *testing.T) {
	s := testService(t, 1, time.Second)
	for _, req := range []Request{{Type: "invalid"}, {Type: ComponentParse, Input: Input{Code: "bad"}}, {Type: QRCodeParse, Input: Input{QRCodeData: "bad"}}, {Type: ComponentParse, Input: Input{Code: "C123", UseLLM: true}}, {Type: ComponentParse, Input: Input{Code: "123456789012"}, Source: "invalid"}} {
		if _, err := s.Submit(req, nil); err == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
	// 无网络的本地平台解析器验证统一结果结构。
	qrReq := request()
	qrReq.Type = QRCodeParse
	qrReq.Input = Input{QRCodeData: "{pc:123456789012,qty:20}"}
	task, err := s.Submit(qrReq, nil)
	if err != nil {
		t.Fatal(err)
	}
	start(t, s)
	done := waitTask(t, s, task.ID, repository.TaskSucceeded)
	var result Result
	if err := json.Unmarshal(done.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Quantity != 20 || result.QRCodeInfo == nil || result.Component.PlatformCode != "123456789012" {
		t.Fatalf("QR result %+v", result)
	}
}
