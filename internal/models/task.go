package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONText 在所有数据库中以 text 存储 JSON，在 HTTP 中保持 JSON 对象。
type JSONText json.RawMessage

func (j JSONText) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}
func (j *JSONText) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case string:
		*j = JSONText(v)
	case []byte:
		*j = append((*j)[:0], v...)
	default:
		return fmt.Errorf("无效 JSON 字段类型 %T", value)
	}
	return nil
}
func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

type Task struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Type       string     `gorm:"size:64;not null;index" json:"type"`
	Input      JSONText   `gorm:"type:text;not null" json:"input"`
	Source     string     `gorm:"size:32" json:"source"`
	RecordID   *uint      `json:"record_id,omitempty"`
	Status     string     `gorm:"size:16;not null;index:idx_tasks_queue,priority:1" json:"status"`
	Result     JSONText   `gorm:"type:text" json:"result"`
	Error      string     `gorm:"type:text" json:"error,omitempty"`
	ErrorCode  string     `gorm:"size:32" json:"error_code,omitempty"`
	RetryOfID  *uint      `json:"retry_of_id,omitempty"`
	CreatedAt  time.Time  `gorm:"index:idx_tasks_queue,priority:2" json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func (j *JSONText) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("无效 JSON")
	}
	*j = append((*j)[:0], data...)
	return nil
}
