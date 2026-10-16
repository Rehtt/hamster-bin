package auth

import (
	"context"
	"crypto/subtle"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const FlowCookieName = "hamster_oauth_flow"
const FlowLifetime = 10 * time.Minute

var ErrOAuthDenied = &oauthFlowError{"access_denied"}

type oauthFlowError struct{ code string }

func (e *oauthFlowError) Error() string { return e.code }

type loginFlow struct {
	State    string
	Nonce    string
	Verifier string
	Expires  time.Time
}

func (s *Service) BeginLogin(ctx context.Context, previousBrowser string) (string, string, error) {
	if !s.cfg.OAuthEnabled {
		return "", "", ErrOAuthUnavailable
	}
	if err := s.discover(ctx); err != nil {
		return "", "", err
	}
	s.flowsMu.Lock()
	defer s.flowsMu.Unlock()
	delete(s.flows, digest(previousBrowser))
	for key, flow := range s.flows {
		if !flow.Expires.After(time.Now()) {
			delete(s.flows, key)
		}
	}
	if len(s.flows) >= 1000 {
		return "", "", ErrOAuthUnavailable
	}
	flow := loginFlow{State: randomValue(), Nonce: randomValue(), Verifier: oauth2.GenerateVerifier(), Expires: time.Now().Add(FlowLifetime)}
	browser := randomValue()
	s.flows[digest(browser)] = flow
	return s.oauthConfig.AuthCodeURL(flow.State, oidc.Nonce(flow.Nonce), oauth2.S256ChallengeOption(flow.Verifier)), browser, nil
}

func (s *Service) CompleteLogin(ctx context.Context, browser, rawQuery string) (string, time.Time, error) {
	if !s.cfg.OAuthEnabled {
		return "", time.Time{}, ErrOAuthUnavailable
	}
	s.flowsMu.Lock()
	flow, found := s.flows[digest(browser)]
	delete(s.flows, digest(browser))
	s.flowsMu.Unlock()
	if browser == "" || !found || !flow.Expires.After(time.Now()) {
		return "", time.Time{}, ErrInvalidToken
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", time.Time{}, ErrInvalidToken
	}
	for _, values := range query {
		if len(values) != 1 {
			return "", time.Time{}, ErrInvalidToken
		}
	}
	if query.Get("state") == "" || subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(flow.State)) != 1 {
		return "", time.Time{}, ErrInvalidToken
	}
	if query.Has("error") {
		if query.Has("code") || query.Get("error") == "" {
			return "", time.Time{}, ErrInvalidToken
		}
		if query.Get("error") == "access_denied" {
			return "", time.Time{}, ErrOAuthDenied
		}
		return "", time.Time{}, ErrOAuthUnavailable
	}
	if query.Get("code") == "" {
		return "", time.Time{}, ErrInvalidToken
	}
	return s.finishLogin(ctx, query.Get("code"), flow)
}
