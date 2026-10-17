package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OAuthRepository struct{ db *gorm.DB }

var ErrOAuthSessionEnded = errors.New("oauth session ended")
var ErrOAuthIDExpired = errors.New("oauth ID token expired")

const OAuthLogoutRetention = 31 * 24 * time.Hour

func NewOAuthRepository(db *gorm.DB) *OAuthRepository { return &OAuthRepository{db: db} }

func (r *OAuthRepository) CreateLogin(ctx context.Context, identity *models.OAuthIdentity, session *models.OAuthSession, idExpiry time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ended int64
		if err := tx.Model(&models.OAuthLogout{}).Where("target_key = ? AND expires_at > ?", session.TargetKey, time.Now()).Count(&ended).Error; err != nil {
			return err
		}
		if ended != 0 || session.ProviderSID == "" || session.TargetKey == "" {
			return ErrOAuthSessionEnded
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"username", "name", "email", "updated_at"}),
		}).Create(identity).Error; err != nil {
			return err
		}
		var stored models.OAuthIdentity
		if err := tx.Where(clause.Eq{Column: clause.Column{Name: "key"}, Value: identity.Key}).First(&stored).Error; err != nil {
			return err
		}
		if stored.Issuer != identity.Issuer || stored.Subject != identity.Subject {
			return errors.New("identity mismatch")
		}
		session.IdentityID = stored.ID
		if !idExpiry.After(time.Now()) {
			return ErrOAuthIDExpired
		}
		return tx.Create(session).Error
	})
}

// EndTarget 与登录落库由 service 的同一目标锁保护；记录和删除必须一起提交。
func (r *OAuthRepository) EndTarget(ctx context.Context, target string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Where("expires_at <= ?", now).Delete(&models.OAuthLogout{}).Error; err != nil {
			return err
		}
		row := models.OAuthLogout{TargetKey: target, ExpiresAt: now.Add(OAuthLogoutRetention)}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "target_key"}}, DoUpdates: clause.AssignmentColumns([]string{"expires_at"})}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Where("target_key = ?", target).Delete(&models.OAuthSession{}).Error
	})
}

func (r *OAuthRepository) Session(ctx context.Context, id string) (*models.OAuthSession, error) {
	var row models.OAuthSession
	err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error
	return &row, err
}

func (r *OAuthRepository) Identity(ctx context.Context, id uint) (*models.OAuthIdentity, error) {
	var row models.OAuthIdentity
	err := r.db.WithContext(ctx).First(&row, id).Error
	return &row, err
}

// BeginRefresh 在网络请求之前提交标记，重启后不能再次使用结果未知的刷新令牌。
func (r *OAuthRepository) BeginRefresh(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Model(&models.OAuthSession{}).Where("id = ? AND refreshing = ?", id, false).Update("refreshing", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("session unavailable")
	}
	return nil
}

func (r *OAuthRepository) CompleteRefresh(ctx context.Context, session *models.OAuthSession) error {
	result := r.db.WithContext(ctx).Model(&models.OAuthSession{}).Where("id = ? AND refreshing = ?", session.ID, true).Updates(map[string]any{
		"tokens": session.Tokens, "access_expires_at": session.AccessExpiresAt, "refreshing": false,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOAuthSessionEnded
	}
	return nil
}

func (r *OAuthRepository) DeleteSession(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.OAuthSession{}, "id = ?", id).Error
}

func (r *OAuthRepository) Cleanup(ctx context.Context, interrupted bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("expires_at <= ?", time.Now()).Or("provider_sid IS NULL OR provider_sid = '' OR target_key IS NULL OR target_key = ''")
		if interrupted {
			query = query.Or("refreshing = ?", true)
		}
		if err := query.Delete(&models.OAuthSession{}).Error; err != nil {
			return err
		}
		return tx.Where("expires_at <= ?", time.Now()).Delete(&models.OAuthLogout{}).Error
	})
}
