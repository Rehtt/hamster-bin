package auth

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type sessionLock struct {
	gate chan struct{}
	refs int
}

type Service struct {
	cfg         *config.Config
	repo        *repository.OAuthRepository
	client      *http.Client
	providerMu  sync.Mutex
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	oauthConfig *oauth2.Config
	revokeURL   string
	jwksURL     string
	flowsMu     sync.Mutex
	flows       map[string]loginFlow
	locksMu     sync.Mutex
	locks       map[string]*sessionLock
}

func NewService(cfg *config.Config, repo *repository.OAuthRepository) (*Service, error) {
	s := &Service{cfg: cfg, repo: repo,
		client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		flows:  make(map[string]loginFlow), locks: make(map[string]*sessionLock),
	}
	if cfg.OAuthEnabled {
		if repo == nil {
			return nil, ErrInvalidToken
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := repo.Cleanup(ctx, true); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Service) lockSession(ctx context.Context, id string) (func(), error) {
	s.locksMu.Lock()
	l := s.locks[id]
	if l == nil {
		l = &sessionLock{gate: make(chan struct{}, 1)}
		s.locks[id] = l
	}
	l.refs++
	s.locksMu.Unlock()
	drop := func() {
		s.locksMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(s.locks, id)
		}
		s.locksMu.Unlock()
	}
	select {
	case l.gate <- struct{}{}:
		return func() { <-l.gate; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

func (s *Service) Authenticate(ctx context.Context, raw string) (*Claims, error) {
	claims, err := ParseToken(raw, s.cfg.JWTSecret)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if claims.AuthMethod == "" || claims.AuthMethod == "password" {
		if !s.cfg.IsPasswordAuthEnabled() || claims.SessionID != "" || claims.Username != s.cfg.AdminUsername {
			return nil, ErrInvalidToken
		}
		claims.AuthMethod = "password"
		return claims, nil
	}
	if claims.AuthMethod != "oauth" || claims.SessionID == "" || !s.cfg.OAuthEnabled {
		return nil, ErrInvalidToken
	}
	id := digest(claims.SessionID)
	unlock, err := s.lockSession(ctx, id)
	if err != nil {
		return nil, ErrInvalidToken
	}
	defer unlock()
	session, err := s.repo.Session(ctx, id)
	if err != nil {
		return nil, ErrInvalidToken
	}
	fail := func() (*Claims, error) { s.invalidate(id); return nil, ErrInvalidToken }
	if session.Refreshing || !session.ExpiresAt.After(time.Now()) || session.ClientID != s.cfg.OAuthClientID || session.ProviderSID == "" {
		return fail()
	}
	identity, err := s.repo.Identity(ctx, session.IdentityID)
	if err != nil || identity.Issuer != s.cfg.OAuthIssuer {
		return fail()
	}
	if session.TargetKey != targetKey(identity.Issuer, session.ClientID, identity.Subject, session.ProviderSID) {
		return fail()
	}
	tokens, err := decryptTokens(s.cfg.JWTSecret, id, session.Tokens)
	if err != nil {
		return fail()
	}
	if err := s.discover(ctx); err != nil {
		return fail()
	}
	if !session.AccessExpiresAt.After(time.Now().Add(30 * time.Second)) {
		if err := s.refresh(ctx, session, identity.Subject, &tokens); err != nil {
			return fail()
		}
	}
	profile, err := s.userInfo(ctx, tokens.Access, identity.Subject)
	if err != nil {
		return fail()
	}
	// UserInfo/刷新在网络等待期间，后端通知可能已删除本地会话。
	current, err := s.repo.Session(ctx, id)
	if err != nil || !current.ExpiresAt.After(time.Now()) {
		return nil, ErrInvalidToken
	}
	claims.Username = profile.displayName()
	return claims, nil
}

// invalidate 不使用可能已取消的请求 context，保证网络超时后仍清理本地会话。
func (s *Service) invalidate(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.repo.DeleteSession(ctx, id)
}

func (s *Service) Logout(ctx context.Context, raw string) error {
	claims, err := ParseToken(raw, s.cfg.JWTSecret)
	if err != nil || claims.AuthMethod != "oauth" || s.repo == nil {
		return nil
	}
	id := digest(claims.SessionID)
	unlock, err := s.lockSession(ctx, id)
	if err != nil {
		return err
	}
	defer unlock()
	session, loadErr := s.repo.Session(ctx, id)
	if err := s.repo.DeleteSession(ctx, id); err != nil {
		return err
	}
	if loadErr != nil || !s.cfg.OAuthEnabled || session.ClientID != s.cfg.OAuthClientID {
		return nil
	}
	tokens, err := decryptTokens(s.cfg.JWTSecret, id, session.Tokens)
	if err == nil && s.discover(ctx) == nil {
		s.revoke(ctx, tokens.Refresh)
	}
	return nil
}
