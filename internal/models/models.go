package models

import (
	"time"
)

// Category 分类表
type Category struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"not null;size:100" json:"name"`
	ParentID *uint  `json:"parent_id,omitempty"` // 父分类ID，支持树形结构
}

// Component 元件表
type Component struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CategoryID    uint      `gorm:"not null;index" json:"category_id"`
	Category      *Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Name          string    `gorm:"not null;size:200" json:"name"`          // 元件名称/型号
	Value         string    `gorm:"size:100" json:"value,omitempty"`        // 参数值(如: 10k, 100nF)
	Package       string    `gorm:"size:50" json:"package,omitempty"`       // 封装形式
	Description   string    `gorm:"type:text" json:"description,omitempty"` // 描述
	StockQuantity int       `gorm:"default:0" json:"stock_quantity"`        // 库存数量
	Location      string    `gorm:"size:100" json:"location,omitempty"`     // 存放位置
	DatasheetURL  string    `gorm:"size:500" json:"datasheet_url,omitempty"`
	ImageURL      string    `gorm:"size:500" json:"image_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// StockLog 库存变更记录表
type StockLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ComponentID  uint      `gorm:"not null;index" json:"component_id"`
	Component    *Component `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	ChangeAmount int       `gorm:"not null" json:"change_amount"` // 正数为入库，负数为出库
	Reason       string    `gorm:"size:500" json:"reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 指定表名
func (Category) TableName() string {
	return "categories"
}

func (Component) TableName() string {
	return "components"
}

func (StockLog) TableName() string {
	return "stock_logs"
}
