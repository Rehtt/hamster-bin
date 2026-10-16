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

func NewOAuthRepository(db *gorm.DB) *OAuthRepository { return &OAuthRepository{db: db} }

func (r *OAuthRepository) CreateLogin(ctx context.Context, identity *models.OAuthIdentity, session *models.OAuthSession) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		return tx.Create(session).Error
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
		return errors.New("session unavailable")
	}
	return nil
}

func (r *OAuthRepository) DeleteSession(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.OAuthSession{}, "id = ?", id).Error
}

func (r *OAuthRepository) Cleanup(ctx context.Context, interrupted bool) error {
	query := r.db.WithContext(ctx).Where("expires_at <= ?", time.Now())
	if interrupted {
		query = query.Or("refreshing = ?", true)
	}
	return query.Delete(&models.OAuthSession{}).Error
}
