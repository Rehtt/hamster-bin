package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type providerGrant struct{ nonce, challenge, subject, sid string }
type fakeProvider struct {
	mu                                                                         sync.Mutex
	server                                                                     *httptest.Server
	key                                                                        *rsa.PrivateKey
	kid, method, clientID, secret                                              string
	codes                                                                      map[string]providerGrant
	refreshes                                                                  map[string]providerGrant
	accesses                                                                   map[string]string
	subject, username, email, infoSubject, sid                                 string
	infoStatus, tokenStatus                                                    int
	codeCalls, refreshCalls, infoCalls, revokeCalls, jwksCalls, counter        int
	modify                                                                     func(jwt.MapClaims)
	badAlgorithm, badSignature, omitRefresh, omitID, omitOffline, stallRefresh bool
}

func newFakeProvider(t *testing.T, method string) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{key: key, kid: "key-1", method: method, clientID: "hamster: client", secret: "secret:+ /", subject: "stable-user", sid: "browser-1", username: "alice", email: "alice@example.com", infoStatus: 200,
		codes: map[string]providerGrant{}, refreshes: map[string]providerGrant{}, accesses: map[string]string{}}
	p.server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.server.Close)
	return p
}

func (p *fakeProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	send := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		send(map[string]any{"issuer": p.server.URL, "authorization_endpoint": p.server.URL + "/authorize", "token_endpoint": p.server.URL + "/token", "userinfo_endpoint": p.server.URL + "/userinfo", "jwks_uri": p.server.URL + "/jwks", "revocation_endpoint": p.server.URL + "/revoke", "id_token_signing_alg_values_supported": []string{"RS256"}})
	case "/jwks":
		p.jwksCalls++
		send(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": p.kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(p.key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.E)).Bytes())}}})
	case "/token", "/revoke":
		if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.ParseForm() != nil {
			w.WriteHeader(400)
			return
		}
		if p.method == "client_secret_basic" {
			id, secret, ok := r.BasicAuth()
			if !ok || id != url.QueryEscape(p.clientID) || secret != url.QueryEscape(p.secret) || r.PostForm.Has("client_secret") || r.PostForm.Has("client_id") {
				w.WriteHeader(401)
				return
			}
		} else if r.Header.Get("Authorization") != "" || r.PostForm.Get("client_id") != p.clientID || r.PostForm.Get("client_secret") != p.secret {
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/revoke" {
			p.revokeCalls++
			delete(p.refreshes, r.PostForm.Get("token"))
			send(map[string]any{})
			return
		}
		var grant providerGrant
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			p.codeCalls++
			code := r.PostForm.Get("code")
			var ok bool
			grant, ok = p.codes[code]
			delete(p.codes, code)
			if !ok || grant.challenge != oauth2.S256ChallengeFromVerifier(r.PostForm.Get("code_verifier")) || r.PostForm.Get("redirect_uri") != "http://127.0.0.1:5173/api/v1/auth/oauth/callback" {
				w.WriteHeader(400)
				return
			}
		case "refresh_token":
			p.refreshCalls++
			refresh := r.PostForm.Get("refresh_token")
			var ok bool
			grant, ok = p.refreshes[refresh]
			delete(p.refreshes, refresh)
			if !ok {
				w.WriteHeader(400)
				return
			}
			// 轮换使旧访问令牌失效，暴露并发使用旧令牌的实现错误。
			clear(p.accesses)
			if p.stallRefresh {
				<-r.Context().Done()
				return
			}
		default:
			w.WriteHeader(400)
			return
		}
		if p.tokenStatus != 0 {
			w.WriteHeader(p.tokenStatus)
			return
		}
		p.counter++
		access, refresh := fmt.Sprintf("opaque-access-%d", p.counter), fmt.Sprintf("opaque-refresh-%d", p.counter)
		p.accesses[access] = grant.subject
		p.refreshes[refresh] = grant
		claims := jwt.MapClaims{"iss": p.server.URL, "sub": grant.subject, "sid": grant.sid, "aud": p.clientID, "iat": time.Now().Unix(), "exp": time.Now().Add(15 * time.Minute).Unix(), "nonce": grant.nonce}
		if p.modify != nil {
			p.modify(claims)
		}
		signing := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		signing.Header["kid"] = p.kid
		var signingKey any = p.key
		if p.badAlgorithm {
			signing = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signingKey = []byte("fake")
		}
		if p.badSignature {
			other, _ := rsa.GenerateKey(rand.Reader, 2048)
			signingKey = other
		}
		signed, err := signing.SignedString(signingKey)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		response := map[string]any{"access_token": access, "refresh_token": refresh, "id_token": signed, "token_type": "Bearer", "expires_in": 900, "scope": "openid profile email offline_access"}
		if p.omitRefresh {
			delete(response, "refresh_token")
		}
		if p.omitID {
			delete(response, "id_token")
		}
		if p.omitOffline {
			response["scope"] = "openid profile email"
		}
		send(response)
	case "/userinfo":
		p.infoCalls++
		subject := p.accesses[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		if p.infoStatus != 200 || subject == "" {
			w.WriteHeader(max(p.infoStatus, 401))
			return
		}
		if p.infoSubject != "" {
			subject = p.infoSubject
		}
		send(map[string]any{"sub": subject, "preferred_username": p.username, "name": "Alice", "email": p.email})
	default:
		w.WriteHeader(404)
	}
}

type oauthFixture struct {
	p       *fakeProvider
	cfg     *config.Config
	db      *gorm.DB
	service *Service
}

func newOAuthFixture(t *testing.T, method string) oauthFixture {
	t.Helper()
	p := newFakeProvider(t, method)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "oauth.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.OAuthIdentity{}, &models.OAuthSession{}, &models.OAuthLogout{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{OAuthEnabled: true, OAuthIssuer: p.server.URL, OAuthClientID: p.clientID, OAuthClientSecret: p.secret, OAuthAuthMethod: method, OAuthRedirectURL: "http://127.0.0.1:5173/api/v1/auth/oauth/callback", JWTSecret: "a-long-random-test-secret-for-jwt", JWTExpireHours: 168}
	service, err := NewService(cfg, repository.NewOAuthRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	return oauthFixture{p, cfg, db, service}
}

func (f oauthFixture) begin(t *testing.T) (string, string) {
	t.Helper()
	location, browser, err := f.service.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("scope") != "openid profile email offline_access" || q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 || q.Get("nonce") == "" || q.Get("state") == "" || q.Has("code_verifier") || q.Has("client_secret") {
		t.Fatalf("invalid authorization request: %s", location)
	}
	f.p.mu.Lock()
	code := randomValue()
	f.p.codes[code] = providerGrant{q.Get("nonce"), q.Get("code_challenge"), f.p.subject, f.p.sid}
	f.p.mu.Unlock()
	return browser, url.Values{"state": {q.Get("state")}, "code": {code}}.Encode()
}

func (f oauthFixture) login(t *testing.T) (string, *Claims) {
	t.Helper()
	browser, query := f.begin(t)
	raw, expires, err := f.service.CompleteLogin(context.Background(), browser, query)
	if err != nil {
		t.Fatal(err)
	}
	if expires.Before(time.Now().Add(167*time.Hour)) || expires.After(time.Now().Add(168*time.Hour)) {
		t.Fatalf("unexpected expiry: %v", expires)
	}
	claims, err := ParseToken(raw, f.cfg.JWTSecret)
	if err != nil || claims.AuthMethod != "oauth" || claims.SessionID == "" {
		t.Fatalf("invalid app token: %v", err)
	}
	return raw, claims
}

func (f oauthFixture) expireAccess(t *testing.T, claims *Claims) {
	t.Helper()
	if err := f.db.Model(&models.OAuthSession{}).Where("id = ?", digest(claims.SessionID)).Update("access_expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
}

func TestOAuthLoginPersistenceAndLogout(t *testing.T) {
	for _, method := range []string{"client_secret_basic", "client_secret_post"} {
		t.Run(method, func(t *testing.T) {
			f := newOAuthFixture(t, method)
			raw, claims := f.login(t)
			row, err := f.service.repo.Session(context.Background(), digest(claims.SessionID))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(row.Tokens), "opaque-") || strings.Contains(raw, "opaque-") {
				t.Fatal("provider tokens leaked")
			}
			if _, err := decryptTokens("different-key", row.ID, row.Tokens); err == nil {
				t.Fatal("wrong encryption key accepted")
			}
			if _, err := decryptTokens(f.cfg.JWTSecret, "different-session", row.Tokens); err == nil {
				t.Fatal("ciphertext not bound to session")
			}
			restored, err := NewService(f.cfg, repository.NewOAuthRepository(f.db))
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				who, err := restored.Authenticate(context.Background(), raw)
				if err != nil || who.Username != "alice" {
					t.Fatalf("restored session: %v", err)
				}
			}
			f.p.mu.Lock()
			if f.p.infoCalls != 3 {
				t.Errorf("checks cached: %d", f.p.infoCalls)
			}
			f.p.username, f.p.email = "renamed", "new@example.com"
			f.p.mu.Unlock()
			nextRaw, next := f.login(t)
			nextRow, _ := f.service.repo.Session(context.Background(), digest(next.SessionID))
			if nextRow.IdentityID != row.IdentityID {
				t.Fatal("profile changes created a new identity")
			}
			f.p.mu.Lock()
			f.p.subject = "different-user"
			f.p.mu.Unlock()
			_, other := f.login(t)
			otherRow, _ := f.service.repo.Session(context.Background(), digest(other.SessionID))
			if otherRow.IdentityID == row.IdentityID {
				t.Fatal("unverified email merged identities")
			}
			if err := restored.Logout(context.Background(), nextRaw); err != nil {
				t.Fatal(err)
			}
			if _, err := restored.Authenticate(context.Background(), nextRaw); err == nil {
				t.Fatal("logged out session accepted")
			}
			f.p.mu.Lock()
			defer f.p.mu.Unlock()
			if f.p.revokeCalls != 1 {
				t.Fatalf("revoke calls=%d", f.p.revokeCalls)
			}
		})
	}
}

func TestOAuthRejectsInvalidIdentity(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*fakeProvider)
	}{
		{"issuer", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { c["iss"] = "https://other.example" } }},
		{"audience", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { c["aud"] = "other-client" } }},
		{"azp_missing", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { c["aud"] = []string{p.clientID, "other"} } }},
		{"azp_wrong", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { c["azp"] = "other-client" } }},
		{"nonce", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { c["nonce"] = "wrong" } }},
		{"nonce_missing", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { delete(c, "nonce") } }},
		{"expired", func(p *fakeProvider) {
			p.modify = func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }
		}},
		{"expiry_missing", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { delete(c, "exp") } }},
		{"iat_future", func(p *fakeProvider) {
			p.modify = func(c jwt.MapClaims) { c["iat"] = time.Now().Add(time.Hour).Unix() }
		}},
		{"iat_missing", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { delete(c, "iat") } }},
		{"subject_empty", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { c["sub"] = "" } }},
		{"sid_missing", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { delete(c, "sid") } }},
		{"sid_empty", func(p *fakeProvider) { p.modify = func(c jwt.MapClaims) { c["sid"] = " " } }},
		{"logout_event", func(p *fakeProvider) {
			p.modify = func(c jwt.MapClaims) { c["events"] = map[string]any{logoutEvent: map[string]any{}} }
		}},
		{"signature", func(p *fakeProvider) { p.badSignature = true }},
		{"algorithm", func(p *fakeProvider) { p.badAlgorithm = true }},
		{"userinfo", func(p *fakeProvider) { p.infoSubject = "different-user" }},
		{"refresh_missing", func(p *fakeProvider) { p.omitRefresh = true }},
		{"offline_denied", func(p *fakeProvider) { p.omitOffline = true }},
		{"id_missing", func(p *fakeProvider) { p.omitID = true }},
		{"wrong_client_secret", func(p *fakeProvider) { p.secret = "rotated" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOAuthFixture(t, "client_secret_basic")
			browser, query := f.begin(t)
			f.p.mu.Lock()
			tc.setup(f.p)
			f.p.mu.Unlock()
			raw, _, err := f.service.CompleteLogin(context.Background(), browser, query)
			if err == nil || raw != "" {
				t.Fatal("invalid identity logged in")
			}
			var count int64
			f.db.Model(&models.OAuthSession{}).Count(&count)
			if count != 0 {
				t.Fatal("failed login saved a session")
			}
			if _, _, err = f.service.CompleteLogin(context.Background(), browser, query); err == nil {
				t.Fatal("callback replay accepted")
			}
		})
	}
}

func TestOAuthCallbackBindingAndSingleUse(t *testing.T) {
	for _, name := range []string{"missing_state", "wrong_state", "duplicate_state", "duplicate_code", "missing_code", "code_and_error", "denied", "missing_browser", "different_browser", "expired", "bad_encoding", "wrong_verifier"} {
		t.Run(name, func(t *testing.T) {
			f := newOAuthFixture(t, "client_secret_basic")
			browser, query := f.begin(t)
			originalBrowser, originalQuery := browser, query
			q, _ := url.ParseQuery(query)
			switch name {
			case "missing_state":
				q.Del("state")
			case "wrong_state":
				q.Set("state", "wrong")
			case "duplicate_state":
				q.Add("state", q.Get("state"))
			case "duplicate_code":
				q.Add("code", q.Get("code"))
			case "missing_code":
				q.Del("code")
			case "code_and_error":
				q.Set("error", "access_denied")
			case "denied":
				q.Del("code")
				q.Set("error", "access_denied")
			case "missing_browser":
				browser = ""
			case "different_browser":
				browser = "different"
			case "expired":
				flow := f.service.flows[digest(browser)]
				flow.Expires = time.Now().Add(-time.Second)
				f.service.flows[digest(browser)] = flow
			case "wrong_verifier":
				flow := f.service.flows[digest(browser)]
				flow.Verifier = oauth2.GenerateVerifier()
				f.service.flows[digest(browser)] = flow
			}
			query = q.Encode()
			if name == "bad_encoding" {
				query += "&code=%ZZ"
			}
			if _, _, err := f.service.CompleteLogin(context.Background(), browser, query); err == nil {
				t.Fatal("invalid callback accepted")
			}
			if name != "missing_browser" && name != "different_browser" {
				if _, _, err := f.service.CompleteLogin(context.Background(), originalBrowser, originalQuery); err == nil {
					t.Fatal("callback was not consumed")
				}
			}
			f.p.mu.Lock()
			defer f.p.mu.Unlock()
			want := 0
			if name == "wrong_verifier" {
				want = 1
			}
			if f.p.codeCalls != want {
				t.Fatalf("code exchanges=%d", f.p.codeCalls)
			}
		})
	}
}

func TestOAuthRefreshSerializedAndDurable(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_post")
	raw, claims := f.login(t)
	f.expireAccess(t, claims)
	var wg sync.WaitGroup
	failures := make(chan error, 12)
	for range 12 {
		wg.Go(func() {
			_, err := f.service.Authenticate(context.Background(), raw)
			if err != nil {
				failures <- err
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	f.p.mu.Lock()
	calls := f.p.refreshCalls
	f.p.mu.Unlock()
	if calls != 1 {
		t.Fatalf("refresh requests=%d", calls)
	}
	row, err := f.service.repo.Session(context.Background(), digest(claims.SessionID))
	if err != nil || row.Refreshing || !row.AccessExpiresAt.After(time.Now()) {
		t.Fatalf("refresh not saved: %v", err)
	}
	tokens, err := decryptTokens(f.cfg.JWTSecret, row.ID, row.Tokens)
	if err != nil || tokens.Refresh != "opaque-refresh-2" {
		t.Fatalf("new refresh token not persisted: %v", err)
	}
	restored, err := NewService(f.cfg, repository.NewOAuthRepository(f.db))
	if err != nil {
		t.Fatal(err)
	}
	f.expireAccess(t, claims)
	if _, err := restored.Authenticate(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if err := restored.Logout(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	f.p.mu.Lock()
	defer f.p.mu.Unlock()
	if _, exists := f.p.refreshes["opaque-refresh-3"]; exists {
		t.Fatal("logout did not revoke latest token")
	}
}

func TestOAuthRevocationFailureAndInterruptedRefresh(t *testing.T) {
	for _, name := range []string{"revoked", "unavailable", "subject_changed", "expired_session", "refresh_failed", "refresh_timeout", "refresh_missing", "refresh_subject", "refresh_sid", "refresh_sid_missing", "refresh_logout", "refresh_save_failed", "interrupted", "secret_changed", "ciphertext_changed"} {
		t.Run(name, func(t *testing.T) {
			f := newOAuthFixture(t, "client_secret_basic")
			raw, claims := f.login(t)
			id := digest(claims.SessionID)
			f.p.mu.Lock()
			switch name {
			case "revoked":
				f.p.infoStatus = 401
			case "unavailable":
				f.p.infoStatus = 503
			case "subject_changed":
				f.p.infoSubject = "another-user"
			case "expired_session":
				f.db.Model(&models.OAuthSession{}).Where("id = ?", id).Update("expires_at", time.Now().Add(-time.Second))
			case "refresh_failed":
				f.p.tokenStatus = 400
				f.expireAccess(t, claims)
			case "refresh_timeout":
				f.p.stallRefresh = true
				f.service.client.Timeout = 30 * time.Millisecond
				f.expireAccess(t, claims)
			case "refresh_missing":
				f.p.omitRefresh = true
				f.expireAccess(t, claims)
			case "refresh_subject":
				f.p.modify = func(c jwt.MapClaims) { c["sub"] = "another-user" }
				f.expireAccess(t, claims)
			case "refresh_sid":
				f.p.modify = func(c jwt.MapClaims) { c["sid"] = "another-browser" }
				f.expireAccess(t, claims)
			case "refresh_sid_missing":
				f.p.modify = func(c jwt.MapClaims) { delete(c, "sid") }
				f.expireAccess(t, claims)
			case "refresh_logout":
				f.p.modify = func(c jwt.MapClaims) { c["events"] = map[string]any{logoutEvent: map[string]any{}} }
				f.expireAccess(t, claims)
			case "refresh_save_failed":
				// 网络成功后无法保存，不能恢复或重试旧令牌。
				if err := f.db.Exec("CREATE TRIGGER fail_rotation BEFORE UPDATE OF tokens ON oauth_sessions BEGIN SELECT RAISE(FAIL, 'disk full'); END").Error; err != nil {
					t.Fatal(err)
				}
				f.expireAccess(t, claims)
			case "interrupted":
				f.db.Model(&models.OAuthSession{}).Where("id = ?", id).Update("refreshing", true)
				var err error
				f.service, err = NewService(f.cfg, repository.NewOAuthRepository(f.db))
				if err != nil {
					t.Fatal(err)
				}
			case "secret_changed":
				f.p.secret = "rotated"
				f.expireAccess(t, claims)
			case "ciphertext_changed":
				f.db.Model(&models.OAuthSession{}).Where("id = ?", id).Update("tokens", []byte("corrupt"))
			}
			f.p.mu.Unlock()
			if _, err := f.service.Authenticate(context.Background(), raw); err == nil {
				t.Fatal("invalid session accepted")
			}
			if _, err := f.service.repo.Session(context.Background(), id); err == nil {
				t.Fatal("invalid session not cleared")
			}
			if _, err := f.service.Authenticate(context.Background(), raw); err == nil {
				t.Fatal("invalid session resurrected")
			}
			f.p.mu.Lock()
			defer f.p.mu.Unlock()
			if f.p.refreshCalls > 1 {
				t.Fatalf("refresh retried: %d", f.p.refreshCalls)
			}
		})
	}
}

func TestOAuthJWKSRotationAndLoginBounds(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	f.login(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f.p.mu.Lock()
	f.p.key = key
	f.p.kid = "key-2"
	f.p.mu.Unlock()
	f.login(t)
	f.p.mu.Lock()
	calls := f.p.jwksCalls
	f.p.mu.Unlock()
	if calls != 2 {
		t.Fatalf("JWKS refreshes=%d", calls)
	}
	// 一个浏览器的新登录会替换旧流程，且过期记录不会无限增长。
	location, browser, err := f.service.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.service.BeginLogin(context.Background(), browser); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(location)
	if _, _, err := f.service.CompleteLogin(context.Background(), browser, "code=x&state="+u.Query().Get("state")); err == nil {
		t.Fatal("replaced flow accepted")
	}
	f.service.flowsMu.Lock()
	for range 1000 {
		f.service.flows[randomValue()] = loginFlow{Expires: time.Now().Add(time.Minute)}
	}
	f.service.flowsMu.Unlock()
	if _, _, err := f.service.BeginLogin(context.Background(), ""); err == nil {
		t.Fatal("pending flow limit ignored")
	}
}

func TestPasswordAuthenticationCannotBypassOAuthOnly(t *testing.T) {
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: "secret", JWTSecret: "test-secret", JWTExpireHours: 1}
	service, err := NewService(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := IssueToken("admin", cfg.JWTSecret, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	cfg.AdminPassword = ""
	if _, err := service.Authenticate(context.Background(), raw); err == nil {
		t.Fatal("disabled password login still accepted")
	}
}

func TestOAuthSessionLifetimeBound(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	// 在转换 time.Duration 前限制小时数，避免极大配置溢出。
	f.cfg.JWTExpireHours = 1_000_000_000
	browser, query := f.begin(t)
	_, expires, err := f.service.CompleteLogin(context.Background(), browser, query)
	if err != nil {
		t.Fatal(err)
	}
	remaining := time.Until(expires)
	if remaining > 30*24*time.Hour || remaining < 30*24*time.Hour-time.Minute {
		t.Fatalf("session exceeded or overflowed 30-day bound: %s", remaining)
	}
}
