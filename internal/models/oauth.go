package models

import "time"

// OAuthIdentity 的 Key 是 (issuer, subject) 的摘要，避免数据库排序规则合并不同身份。
type OAuthIdentity struct {
	ID        uint   `gorm:"primaryKey"`
	Key       string `gorm:"size:64;not null;uniqueIndex"`
	Issuer    string `gorm:"type:text;not null"`
	Subject   string `gorm:"type:text;not null"`
	Username  string `gorm:"type:text"`
	Name      string `gorm:"type:text"`
	Email     string `gorm:"type:text"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OAuthSession 仅保存随机会话 ID 的摘要及经过认证加密的令牌包。
type OAuthSession struct {
	ID              string    `gorm:"primaryKey;size:64"`
	IdentityID      uint      `gorm:"not null;index"`
	ClientID        string    `gorm:"type:text;not null"`
	Tokens          []byte    `gorm:"not null" json:"-"`
	AccessExpiresAt time.Time `gorm:"not null"`
	ExpiresAt       time.Time `gorm:"not null;index"`
	Refreshing      bool      `gorm:"not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (OAuthIdentity) TableName() string { return "oauth_identities" }
func (OAuthSession) TableName() string  { return "oauth_sessions" }
