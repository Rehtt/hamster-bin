package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var ErrOAuthUnavailable = errors.New("统一登录暂不可用")

type oauthProfile struct {
	Subject  string `json:"sub"`
	Username string `json:"preferred_username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

func (p oauthProfile) displayName() string {
	for _, v := range []string{p.Username, p.Name, p.Subject} {
		if v != "" {
			return v
		}
	}
	return ""
}

func (s *Service) discover(ctx context.Context) error {
	s.providerMu.Lock()
	defer s.providerMu.Unlock()
	if s.provider != nil {
		return nil
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, s.client), s.cfg.OAuthIssuer)
	if err != nil {
		return ErrOAuthUnavailable
	}
	var metadata struct {
		RevokeURL string `json:"revocation_endpoint"`
		JWKSURL   string `json:"jwks_uri"`
	}
	if provider.Claims(&metadata) != nil || metadata.RevokeURL == "" || metadata.JWKSURL == "" {
		return ErrOAuthUnavailable
	}
	endpoint := provider.Endpoint()
	endpoint.AuthStyle = oauth2.AuthStyleInHeader
	if s.cfg.OAuthAuthMethod == "client_secret_post" {
		endpoint.AuthStyle = oauth2.AuthStyleInParams
	}
	s.oauthConfig = &oauth2.Config{ClientID: s.cfg.OAuthClientID, ClientSecret: s.cfg.OAuthClientSecret,
		RedirectURL: s.cfg.OAuthRedirectURL, Endpoint: endpoint, Scopes: []string{"openid", "profile", "email", "offline_access"},
	}
	s.verifier = provider.VerifierContext(oidc.ClientContext(context.Background(), s.client), &oidc.Config{
		ClientID: s.cfg.OAuthClientID, SupportedSigningAlgs: []string{oidc.RS256},
	})
	s.revokeURL = metadata.RevokeURL
	s.jwksURL = metadata.JWKSURL
	s.provider = provider
	return nil
}

type oauthID struct {
	*oidc.IDToken
	SessionID string
}

func (s *Service) verifyID(ctx context.Context, raw, nonce, subject string, requireNonce bool) (*oauthID, error) {
	header, err := tokenHeader(raw)
	if err != nil || header.Type == "logout+jwt" {
		return nil, ErrInvalidToken
	}
	token, err := s.verifier.Verify(ctx, raw)
	if err != nil || token.Subject == "" || token.Issuer != s.cfg.OAuthIssuer || token.IssuedAt.IsZero() || token.Expiry.IsZero() || token.IssuedAt.After(time.Now().Add(time.Minute)) || !token.IssuedAt.Before(token.Expiry) {
		return nil, ErrInvalidToken
	}
	if subject != "" && token.Subject != subject {
		return nil, ErrInvalidToken
	}
	if requireNonce || token.Nonce != "" {
		if nonce == "" || subtle.ConstantTimeCompare([]byte(token.Nonce), []byte(nonce)) != 1 {
			return nil, ErrInvalidToken
		}
	}
	var claims struct {
		AuthorizedParty string                     `json:"azp"`
		SessionID       string                     `json:"sid"`
		Events          map[string]json.RawMessage `json:"events"`
	}
	if token.Claims(&claims) != nil || strings.TrimSpace(claims.SessionID) == "" || (len(token.Audience) > 1 && claims.AuthorizedParty == "") || (claims.AuthorizedParty != "" && claims.AuthorizedParty != s.cfg.OAuthClientID) {
		return nil, ErrInvalidToken
	}
	if _, logout := claims.Events[logoutEvent]; logout {
		return nil, ErrInvalidToken
	}
	return &oauthID{token, claims.SessionID}, nil
}

func (s *Service) userInfo(ctx context.Context, access, subject string) (*oauthProfile, error) {
	info, err := s.provider.UserInfo(oidc.ClientContext(ctx, s.client), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: access, TokenType: "Bearer"}))
	if err != nil || info.Subject != subject {
		return nil, ErrInvalidToken
	}
	var profile oauthProfile
	if info.Claims(&profile) != nil || profile.Subject != subject {
		return nil, ErrInvalidToken
	}
	return &profile, nil
}

// tokenResponse 不采用 TokenSource 的隐式刷新，显式处理一次性轮换与持久化。
type tokenResponse struct {
	Access    string `json:"access_token"`
	Refresh   string `json:"refresh_token"`
	ID        string `json:"id_token"`
	Type      string `json:"token_type"`
	ExpiresIn int64  `json:"expires_in"`
	Scope     string `json:"scope"`
}

func (s *Service) clientPost(ctx context.Context, endpoint string, values url.Values, out any) error {
	if s.cfg.OAuthAuthMethod == "client_secret_post" {
		values.Set("client_id", s.cfg.OAuthClientID)
		values.Set("client_secret", s.cfg.OAuthClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return ErrOAuthUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if s.cfg.OAuthAuthMethod == "client_secret_basic" {
		req.SetBasicAuth(url.QueryEscape(s.cfg.OAuthClientID), url.QueryEscape(s.cfg.OAuthClientSecret))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return ErrOAuthUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrOAuthUnavailable
	}
	if out == nil {
		return nil
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out) != nil {
		return ErrOAuthUnavailable
	}
	return nil
}

func validTokenResponse(tokens tokenResponse) bool {
	return tokens.Access != "" && tokens.Refresh != "" && strings.EqualFold(tokens.Type, "Bearer") && tokens.ExpiresIn > 0 && tokens.ExpiresIn <= int64((30*24*time.Hour)/time.Second)
}

func (s *Service) refresh(ctx context.Context, session *models.OAuthSession, subject string, stored *oauthTokens) error {
	if stored.Refresh == "" {
		return ErrInvalidToken
	}
	if err := s.repo.BeginRefresh(ctx, session.ID); err != nil {
		return ErrInvalidToken
	}
	var response tokenResponse
	if s.clientPost(ctx, s.oauthConfig.Endpoint.TokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {stored.Refresh}}, &response) != nil {
		return ErrInvalidToken
	}
	if !validTokenResponse(response) || response.Refresh == stored.Refresh {
		return ErrInvalidToken
	}
	if response.ID != "" {
		id, err := s.verifyID(ctx, response.ID, stored.Nonce, subject, false)
		if err != nil || id.SessionID != session.ProviderSID {
			s.revoke(ctx, response.Refresh)
			return ErrInvalidToken
		}
	}
	updated := oauthTokens{Access: response.Access, Refresh: response.Refresh, Nonce: stored.Nonce}
	encrypted, err := encryptTokens(s.cfg.JWTSecret, session.ID, updated)
	if err != nil {
		s.revoke(ctx, response.Refresh)
		return ErrInvalidToken
	}
	session.Tokens = encrypted
	session.AccessExpiresAt = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)
	if err := s.repo.CompleteRefresh(ctx, session); err != nil {
		// 通知只结束本地会话；已删除的目标不能恢复，也不额外撤销离线授权。
		if !errors.Is(err, repository.ErrOAuthSessionEnded) {
			s.revoke(context.WithoutCancel(ctx), response.Refresh)
		}
		return ErrInvalidToken
	}
	*stored = updated
	return nil
}

func (s *Service) revoke(ctx context.Context, refresh string) {
	if refresh != "" {
		_ = s.clientPost(ctx, s.revokeURL, url.Values{"token": {refresh}, "token_type_hint": {"refresh_token"}}, nil)
	}
}

func (s *Service) finishLogin(ctx context.Context, code string, flow loginFlow) (string, time.Time, error) {
	if err := s.discover(ctx); err != nil {
		return "", time.Time{}, err
	}
	var response tokenResponse
	if s.clientPost(ctx, s.oauthConfig.Endpoint.TokenURL, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {s.cfg.OAuthRedirectURL}, "code_verifier": {flow.Verifier},
	}, &response) != nil {
		return "", time.Time{}, ErrOAuthUnavailable
	}
	revokeOnFailure := true
	defer func() {
		if revokeOnFailure {
			s.revoke(context.WithoutCancel(ctx), response.Refresh)
		}
	}()
	if !validTokenResponse(response) || response.ID == "" {
		return "", time.Time{}, ErrInvalidToken
	}
	if response.Scope != "" && (!slices.Contains(strings.Fields(response.Scope), "openid") || !slices.Contains(strings.Fields(response.Scope), "offline_access")) {
		return "", time.Time{}, ErrInvalidToken
	}
	id, err := s.verifyID(ctx, response.ID, flow.Nonce, "", true)
	if err != nil {
		return "", time.Time{}, ErrInvalidToken
	}
	profile, err := s.userInfo(ctx, response.Access, id.Subject)
	if err != nil {
		return "", time.Time{}, ErrInvalidToken
	}
	identity := &models.OAuthIdentity{Key: identityKey(id.Issuer, id.Subject), Issuer: id.Issuer, Subject: id.Subject, Username: profile.Username, Name: profile.Name, Email: profile.Email}
	expires := time.Now().Add(time.Duration(min(s.cfg.JWTExpireHours, 30*24)) * time.Hour)
	sessionID := randomValue()
	key := digest(sessionID)
	encrypted, err := encryptTokens(s.cfg.JWTSecret, key, oauthTokens{Access: response.Access, Refresh: response.Refresh, Nonce: flow.Nonce})
	if err != nil {
		return "", time.Time{}, err
	}
	target := targetKey(id.Issuer, s.cfg.OAuthClientID, id.Subject, id.SessionID)
	session := &models.OAuthSession{ID: key, ClientID: s.cfg.OAuthClientID, ProviderSID: id.SessionID, TargetKey: target, Tokens: encrypted, AccessExpiresAt: time.Now().Add(time.Duration(response.ExpiresIn) * time.Second), ExpiresAt: expires}
	localToken, err := IssueOAuthToken(profile.displayName(), sessionID, s.cfg.JWTSecret, expires)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := s.repo.Cleanup(ctx, false); err != nil {
		return "", time.Time{}, ErrOAuthUnavailable
	}
	unlock, err := s.lockSession(ctx, "target:"+target)
	if err != nil {
		return "", time.Time{}, ErrOAuthUnavailable
	}
	defer unlock()
	if err := s.repo.CreateLogin(ctx, identity, session, id.Expiry); err != nil {
		if errors.Is(err, repository.ErrOAuthSessionEnded) {
			revokeOnFailure = false
			return "", time.Time{}, ErrInvalidToken
		}
		if errors.Is(err, repository.ErrOAuthIDExpired) {
			return "", time.Time{}, ErrInvalidToken
		}
		return "", time.Time{}, ErrOAuthUnavailable
	}
	revokeOnFailure = false
	return localToken, expires, nil
}
