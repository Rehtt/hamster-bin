package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/llm"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"gorm.io/gorm"
)

func photoService(t *testing.T) (*Service, *gorm.DB, *ImageInput) {
	t.Helper()
	s, db := testServiceWithDB(t, 1, time.Second)
	s.images = NewImageStore(t.TempDir())
	s.imageParser = parser.NewImageParser(llm.NewClient("http://localhost", "key", "model"))
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	photo, err := s.SubmitImage(Request{Source: "components"}, "test.png", data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Cancel(photo.ID); err != nil {
		t.Fatal(err)
	}
	var input Input
	if err := json.Unmarshal(photo.Input, &input); err != nil {
		t.Fatal(err)
	}
	return s, db, input.Image
}

func TestDeleteSharedPhotoAndLastReference(t *testing.T) {
	s, _, input := photoService(t)
	retry, err := s.Retry(1)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.DeleteOne(1)
	if err != nil || result.DeletedCount != 1 || result.ImageCleanupFailedCount != 0 {
		t.Fatalf("delete: %+v %v", result, err)
	}
	if _, err := s.images.path(input); err != nil {
		t.Fatalf("shared image removed: %v", err)
	}
	stored, err := s.Repo.Get(retry.ID)
	if err != nil || stored.RetryOfID == nil || *stored.RetryOfID != 1 {
		t.Fatalf("retry history lost: %+v %v", stored, err)
	}
	if err := s.Cancel(retry.ID); err != nil {
		t.Fatal(err)
	}
	result, err = s.DeleteOne(retry.ID)
	if err != nil || result.ImageCleanupFailedCount != 0 {
		t.Fatalf("last delete: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(s.images.dir, input.Key)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("last reference image retained: %v", err)
	}
}

func TestDeleteImageCleanupFailures(t *testing.T) {
	for _, scenario := range []string{"missing", "remove_failure", "invalid_candidate", "invalid_remaining", "reference_query_failure", "rollback"} {
		t.Run(scenario, func(t *testing.T) {
			s, db, input := photoService(t)
			path := filepath.Join(s.images.dir, input.Key)
			wantFailed := 1
			switch scenario {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				wantFailed = 0
			case "remove_failure":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "keep"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "invalid_candidate":
				if err := db.Model(&models.Task{}).Where("id = 1").Update("input", `{"image":{"key":"../unsafe"}}`).Error; err != nil {
					t.Fatal(err)
				}
			case "invalid_remaining":
				if err := s.Repo.Create(&models.Task{Type: ImageParse, Input: models.JSONText(`{}`), Status: repository.TaskQueued}); err != nil {
					t.Fatal(err)
				}
			case "reference_query_failure":
				if err := db.Callback().Query().Before("gorm:query").Register("test:references", func(tx *gorm.DB) {
					if len(tx.Statement.Selects) > 0 {
						tx.AddError(errors.New("reference query failed"))
					}
				}); err != nil {
					t.Fatal(err)
				}
			case "rollback":
				if err := db.Exec(`CREATE TRIGGER no_delete BEFORE DELETE ON tasks BEGIN SELECT RAISE(ABORT, 'rollback'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			result, err := s.DeleteOne(1)
			if scenario == "rollback" {
				if err == nil {
					t.Fatal("expected rollback")
				}
				if _, err := s.Repo.Get(1); err != nil {
					t.Fatal(err)
				}
			} else if err != nil || result.DeletedCount != 1 || result.ImageCleanupFailedCount != wantFailed {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if scenario != "missing" {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("file should remain: %v", err)
				}
			}
		})
	}
}

func TestRetryDeletePhotoConcurrency(t *testing.T) {
	for range 12 {
		s, _, input := photoService(t)
		var wg sync.WaitGroup
		var retry *models.Task
		var retryErr, deleteErr error
		gate := make(chan struct{})
		wg.Go(func() { <-gate; retry, retryErr = s.Retry(1) })
		wg.Go(func() { <-gate; _, deleteErr = s.DeleteOne(1) })
		close(gate)
		wg.Wait()
		if deleteErr != nil {
			t.Fatal(deleteErr)
		}
		if retryErr == nil {
			if retry == nil {
				t.Fatal("missing retry")
			}
			if _, err := s.images.path(input); err != nil {
				t.Fatalf("retry lost image: %v", err)
			}
		} else {
			if !errors.Is(retryErr, gorm.ErrRecordNotFound) {
				t.Fatal(retryErr)
			}
			if _, err := os.Stat(filepath.Join(s.images.dir, input.Key)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unreferenced image retained: %v", err)
			}
		}
	}
}

func TestCancelledDeletedTaskIgnoresLateExecutor(t *testing.T) {
	s := testService(t, 1, time.Second)
	started, release := make(chan struct{}), make(chan struct{})
	s.execute = func(context.Context, string, Input) (*Result, error) {
		close(started)
		<-release
		return &Result{}, nil
	}
	task := submit(t, s)
	start(t, s)
	<-started
	if err := s.Cancel(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteOne(task.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	s.Stop()
	if _, err := s.Repo.Get(task.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("late executor restored deleted task: %v", err)
	}
}

func TestDeleteSelectedDeduplicatesAndSkips(t *testing.T) {
	s := testService(t, 1, time.Second)
	task := submit(t, s)
	if err := s.Cancel(task.ID); err != nil {
		t.Fatal(err)
	}
	active := submit(t, s)
	result, err := s.Delete(DeleteRequest{Mode: repository.DeleteSelected, IDs: []uint{task.ID, task.ID, active.ID, 9999, 9999}})
	if err != nil || result.DeletedCount != 1 || result.SkippedCount != 2 || len(result.DeletedIDs) != 1 {
		t.Fatalf("dedup result=%+v err=%v", result, err)
	}
}
