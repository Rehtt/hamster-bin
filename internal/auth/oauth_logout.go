package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
)

const logoutEvent = "http://schemas.openid.net/event/backchannel-logout"

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

func tokenHeader(raw string) (jwtHeader, error) {
	var header jwtHeader
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return header, ErrInvalidToken
	}
	encoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(encoded, &header) != nil {
		return header, ErrInvalidToken
	}
	return header, nil
}

// logoutKeys 每次通知读取可信发现文档中的 JWKS，不使用 JWT 内的 jku/x5u/jwk。
// 显式获取使暂时的网络故障与非法签名可以分别返回 503 和 400。
func (s *Service) logoutKeys(ctx context.Context, kid string) (*oidc.StaticKeySet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.jwksURL, nil)
	if err != nil {
		return nil, ErrOAuthUnavailable
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, ErrOAuthUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrOAuthUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	var jwks jose.JSONWebKeySet
	if err != nil || len(body) > 1<<20 || json.Unmarshal(body, &jwks) != nil {
		return nil, ErrOAuthUnavailable
	}
	keys := &oidc.StaticKeySet{}
	for _, key := range jwks.Keys {
		if public, ok := key.Key.(*rsa.PublicKey); ok && key.KeyID == kid && (key.Algorithm == "" || key.Algorithm == oidc.RS256) && (key.Use == "" || key.Use == "sig") {
			keys.PublicKeys = append(keys.PublicKeys, public)
		}
	}
	if len(keys.PublicKeys) == 0 {
		return nil, ErrInvalidToken
	}
	return keys, nil
}

func (s *Service) BackchannelLogout(ctx context.Context, raw string) error {
	if !s.cfg.OAuthEnabled {
		return ErrOAuthUnavailable
	}
	header, err := tokenHeader(raw)
	if err != nil || header.Algorithm != oidc.RS256 || strings.TrimSpace(header.KeyID) == "" || header.Type != "logout+jwt" {
		return ErrInvalidToken
	}
	if err := s.discover(ctx); err != nil {
		return err
	}
	keys, err := s.logoutKeys(ctx, header.KeyID)
	if err != nil {
		return err
	}
	verifier := oidc.NewVerifier(s.cfg.OAuthIssuer, keys, &oidc.Config{ClientID: s.cfg.OAuthClientID, SupportedSigningAlgs: []string{oidc.RS256}})
	token, err := verifier.VerifyLogout(ctx, raw)
	if err != nil {
		return ErrInvalidToken
	}
	now := time.Now()
	if token.Issuer != s.cfg.OAuthIssuer || strings.TrimSpace(token.Subject) == "" || strings.TrimSpace(token.SessionID) == "" || strings.TrimSpace(token.TokenID) == "" || token.IssuedAt.IsZero() || token.IssuedAt.After(now.Add(time.Minute)) || token.IssuedAt.Before(now.Add(-3*time.Minute)) || !token.Expiry.After(now) || !token.Expiry.After(token.IssuedAt) {
		return ErrInvalidToken
	}
	var claims struct {
		Events map[string]json.RawMessage `json:"events"`
		// go-oidc 也接受字符串时间，这里收紧为 JSON 数值。
		IssuedAt float64 `json:"iat"`
		Expiry   float64 `json:"exp"`
	}
	if token.Claims(&claims) != nil {
		return ErrInvalidToken
	}
	var event map[string]json.RawMessage
	if json.Unmarshal(claims.Events[logoutEvent], &event) != nil || event == nil {
		return ErrInvalidToken
	}
	target := targetKey(token.Issuer, s.cfg.OAuthClientID, token.Subject, token.SessionID)
	unlock, err := s.lockSession(ctx, "target:"+target)
	if err != nil {
		return ErrOAuthUnavailable
	}
	defer unlock()
	if s.repo.EndTarget(ctx, target) != nil {
		return ErrOAuthUnavailable
	}
	return nil
}
