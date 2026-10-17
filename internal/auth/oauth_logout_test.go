package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/golang-jwt/jwt/v5"
)

func (f oauthFixture) logoutToken(t *testing.T, modify func(*jwt.Token)) string {
	t.Helper()
	f.p.mu.Lock()
	defer f.p.mu.Unlock()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": f.p.server.URL, "aud": f.p.clientID, "sub": f.p.subject, "sid": f.p.sid,
		"iat": time.Now().Unix(), "exp": time.Now().Add(2 * time.Minute).Unix(), "jti": randomValue(),
		"events": map[string]any{logoutEvent: map[string]any{}},
	})
	token.Header["typ"], token.Header["kid"] = "logout+jwt", f.p.kid
	if modify != nil {
		modify(token)
	}
	var key any = f.p.key
	if strings.HasPrefix(token.Method.Alg(), "HS") {
		key = []byte("wrong-secret")
	}
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBackchannelLogoutTargetsAndPersistence(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	first, firstClaims := f.login(t)
	second, _ := f.login(t)
	logout := f.logoutToken(t, nil)
	f.p.mu.Lock()
	f.p.sid = "browser-2"
	f.p.mu.Unlock()
	otherDevice, _ := f.login(t)
	f.p.mu.Lock()
	f.p.sid, f.p.subject = "browser-1", "another-user"
	f.p.mu.Unlock()
	otherUser, _ := f.login(t)
	// 相同提供方目标在不同 client/issuer 下也不能被删除。
	original, err := f.service.repo.Session(context.Background(), digest(firstClaims.SessionID))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{targetKey(f.cfg.OAuthIssuer, "other-client", "stable-user", "browser-1"), targetKey("other-issuer", f.cfg.OAuthClientID, "stable-user", "browser-1")} {
		copy := *original
		copy.ID, copy.TargetKey = digest(randomValue()), target
		if err := f.db.Create(&copy).Error; err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := f.service.BackchannelLogout(context.Background(), logout); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{first, second} {
		if _, err := f.service.Authenticate(context.Background(), raw); err == nil {
			t.Fatal("ended session accepted")
		}
	}
	for _, raw := range []string{otherDevice, otherUser} {
		if _, err := f.service.Authenticate(context.Background(), raw); err != nil {
			t.Fatalf("unrelated session ended: %v", err)
		}
	}
	var sessions, records int64
	f.db.Model(&models.OAuthSession{}).Count(&sessions)
	f.db.Model(&models.OAuthLogout{}).Count(&records)
	if sessions != 4 || records != 1 {
		t.Fatalf("sessions=%d records=%d", sessions, records)
	}
	f.p.mu.Lock()
	f.p.subject = "stable-user"
	f.p.mu.Unlock()
	if err := f.service.BackchannelLogout(context.Background(), f.logoutToken(t, nil)); err != nil {
		t.Fatal(err)
	}
	var record models.OAuthLogout
	if err := f.db.First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if time.Until(record.ExpiresAt) < repository.OAuthLogoutRetention-time.Minute {
		t.Fatal("logout retention too short")
	}
	// 重建 service，结束状态必须从持久化存储恢复。
	f.service, err = NewService(f.cfg, repository.NewOAuthRepository(f.db))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Authenticate(context.Background(), first); err == nil {
		t.Fatal("restart restored ended session")
	}
	browser, query := f.begin(t)
	if raw, _, err := f.service.CompleteLogin(context.Background(), browser, query); !errors.Is(err, ErrInvalidToken) || raw != "" {
		t.Fatalf("late login accepted: %v", err)
	}
	unknown := f.logoutToken(t, func(token *jwt.Token) { token.Claims.(jwt.MapClaims)["sid"] = "unknown-device" })
	if err := f.service.BackchannelLogout(context.Background(), unknown); err != nil {
		t.Fatalf("unknown target: %v", err)
	}
}

func TestBackchannelLogoutRejectsInvalidTokens(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	rawSession, _ := f.login(t)
	type tokenCase struct {
		name   string
		modify func(*jwt.Token)
	}
	cases := []tokenCase{
		{"issuer", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["iss"] = "https://other.example" }},
		{"issuer_missing", func(t *jwt.Token) { delete(t.Claims.(jwt.MapClaims), "iss") }},
		{"audience", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["aud"] = "other-client" }},
		{"audience_missing", func(t *jwt.Token) { delete(t.Claims.(jwt.MapClaims), "aud") }},
		{"algorithm", func(t *jwt.Token) { t.Method = jwt.SigningMethodHS256 }},
		{"ps256", func(t *jwt.Token) { t.Method = jwt.SigningMethodPS256 }},
		{"kid_unknown", func(t *jwt.Token) { t.Header["kid"] = "other-key" }},
		{"kid_missing", func(t *jwt.Token) { delete(t.Header, "kid") }},
		{"typ_wrong", func(t *jwt.Token) { t.Header["typ"] = "JWT" }},
		{"typ_missing", func(t *jwt.Token) { delete(t.Header, "typ") }},
		{"iat_old", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["iat"] = time.Now().Add(-4 * time.Minute).Unix() }},
		{"iat_future", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["iat"] = time.Now().Add(2 * time.Minute).Unix() }},
		{"iat_string", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["iat"] = strconv.FormatInt(time.Now().Unix(), 10) }},
		{"exp_string", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["exp"] = "9999999999" }},
		{"expired", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["exp"] = time.Now().Add(-time.Second).Unix() }},
		{"exp_before_iat", func(t *jwt.Token) {
			t.Claims.(jwt.MapClaims)["iat"] = time.Now().Add(30 * time.Second).Unix()
			t.Claims.(jwt.MapClaims)["exp"] = time.Now().Add(20 * time.Second).Unix()
		}},
		{"exp_equal_iat", func(t *jwt.Token) {
			t.Claims.(jwt.MapClaims)["iat"] = time.Now().Add(30 * time.Second).Unix()
			t.Claims.(jwt.MapClaims)["exp"] = t.Claims.(jwt.MapClaims)["iat"]
		}},
		{"events_missing", func(t *jwt.Token) { delete(t.Claims.(jwt.MapClaims), "events") }},
		{"events_null", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["events"] = nil }},
		{"events_array", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["events"] = []any{} }},
		{"nonce_object", func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["nonce"] = map[string]any{} }},
		{"event_wrong", func(t *jwt.Token) {
			t.Claims.(jwt.MapClaims)["events"] = map[string]any{"other-event": map[string]any{}}
		}},
	}
	for _, field := range []string{"sub", "sid", "jti", "iat", "exp"} {
		cases = append(cases, tokenCase{field + "_missing", func(t *jwt.Token) { delete(t.Claims.(jwt.MapClaims), field) }})
		for name, value := range map[string]any{"empty": "", "null": nil, "array": []any{}} {
			cases = append(cases, tokenCase{field + "_" + name, func(t *jwt.Token) { t.Claims.(jwt.MapClaims)[field] = value }})
		}
	}
	for name, value := range map[string]any{"null": nil, "string": "", "number": 1, "array": []any{}, "bool": true} {
		cases = append(cases, tokenCase{"event_" + name, func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["events"] = map[string]any{logoutEvent: value} }})
		cases = append(cases, tokenCase{"nonce_" + name, func(t *jwt.Token) { t.Claims.(jwt.MapClaims)["nonce"] = value }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := f.service.BackchannelLogout(context.Background(), f.logoutToken(t, tc.modify)); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("invalid logout error: %v", err)
			}
		})
	}
	valid := f.logoutToken(t, nil)
	parts := strings.Split(valid, ".")
	parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 256))
	for _, raw := range []string{"invalid", strings.Join(parts, "."), "a.b.c", "e30.e30."} {
		if err := f.service.BackchannelLogout(context.Background(), raw); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("bad signature/format: %v", err)
		}
	}
	if _, err := f.service.Authenticate(context.Background(), rawSession); err != nil {
		t.Fatal("invalid notification ended session")
	}
	var count int64
	f.db.Model(&models.OAuthLogout{}).Count(&count)
	if count != 0 {
		t.Fatal("invalid notification persisted")
	}
}

type logoutTransport func(*http.Request) (*http.Response, error)

func (f logoutTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// 在响应到达应用之前暂停，避免以 sleep 猜测并发请求的先后顺序。
func pauseOAuthResponse(t *testing.T, s *Service, path string) (<-chan struct{}, func()) {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.client.Transport = logoutTransport(func(r *http.Request) (*http.Response, error) {
		resp, err := http.DefaultTransport.RoundTrip(r)
		if r.URL.Path == path {
			once.Do(func() { close(entered); <-release })
		}
		return resp, err
	})
	var resumeOnce sync.Once
	resume := func() { resumeOnce.Do(func() { close(release) }) }
	t.Cleanup(resume)
	return entered, resume
}

func awaitOAuth[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("OAuth operation stalled")
		var zero T
		return zero
	}
}

func TestBackchannelLogoutConcurrentOnlineRequests(t *testing.T) {
	for _, path := range []string{"/userinfo", "/token"} {
		t.Run(path, func(t *testing.T) {
			f := newOAuthFixture(t, "client_secret_basic")
			raw, claims := f.login(t)
			if path == "/token" {
				f.expireAccess(t, claims)
			}
			entered, resume := pauseOAuthResponse(t, f.service, path)
			done := make(chan error, 1)
			go func() { _, err := f.service.Authenticate(context.Background(), raw); done <- err }()
			awaitOAuth(t, entered)
			if err := f.service.BackchannelLogout(context.Background(), f.logoutToken(t, nil)); err != nil {
				t.Fatal(err)
			}
			resume()
			if err := awaitOAuth(t, done); err == nil {
				t.Fatal("inflight request accepted after logout")
			}
			if _, err := f.service.repo.Session(context.Background(), digest(claims.SessionID)); err == nil {
				t.Fatal("refresh recreated deleted session")
			}
			f.p.mu.Lock()
			defer f.p.mu.Unlock()
			if f.p.revokeCalls != 0 {
				t.Fatal("concurrent backchannel logout revoked offline authorization")
			}
		})
	}
}

func TestBackchannelLogoutBeforeLoginCommit(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	browser, query := f.begin(t)
	entered, resume := pauseOAuthResponse(t, f.service, "/userinfo")
	done := make(chan error, 1)
	go func() { _, _, err := f.service.CompleteLogin(context.Background(), browser, query); done <- err }()
	awaitOAuth(t, entered)
	if err := f.service.BackchannelLogout(context.Background(), f.logoutToken(t, nil)); err != nil {
		t.Fatal(err)
	}
	resume()
	if err := awaitOAuth(t, done); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("late login: %v", err)
	}
	var count int64
	f.db.Model(&models.OAuthSession{}).Count(&count)
	if count != 0 {
		t.Fatal("late callback created session")
	}
	f.p.mu.Lock()
	defer f.p.mu.Unlock()
	if f.p.revokeCalls != 0 {
		t.Fatal("late callback revoked offline authorization after backchannel logout")
	}
}

func TestBackchannelLogoutFreshJWKSAndTrustedSource(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	f.login(t) // 登录已缓存原公钥。
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f.p.mu.Lock()
	untrustedKey := f.p.key
	f.p.key, f.p.kid = key, "rotated-key"
	f.p.mu.Unlock()
	raw := f.logoutToken(t, func(token *jwt.Token) {
		token.Header["jku"], token.Header["x5u"] = "http://127.0.0.1:1/untrusted", "http://127.0.0.1:1/untrusted"
		token.Header["jwk"] = map[string]any{"kty": "RSA", "kid": "untrusted", "n": base64.RawURLEncoding.EncodeToString(untrustedKey.N.Bytes()), "e": "AQAB"}
		token.Claims.(jwt.MapClaims)["aud"] = []string{"other-client", f.cfg.OAuthClientID}
		token.Claims.(jwt.MapClaims)["iat"] = time.Now().Add(45 * time.Second).Unix()
	})
	for range 2 {
		if err := f.service.BackchannelLogout(context.Background(), raw); err != nil {
			t.Fatal(err)
		}
	}
	f.p.mu.Lock()
	defer f.p.mu.Unlock()
	if f.p.jwksCalls != 3 {
		t.Fatalf("logout used cached/untrusted keys: %d", f.p.jwksCalls)
	}
}

func TestOAuthIDRejectsLogoutType(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	if err := f.service.discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw := f.logoutToken(t, func(token *jwt.Token) { delete(token.Claims.(jwt.MapClaims), "events") })
	if _, err := f.service.verifyID(context.Background(), raw, "", "", false); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("logout typ accepted as ID token")
	}
}

func TestBackchannelLogoutTemporaryFailures(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		status           int
	}{
		{"discovery_network", "/.well-known/openid-configuration", "", 0},
		{"discovery_status", "/.well-known/openid-configuration", "", 503},
		{"jwks_network", "/jwks", "", 0},
		{"jwks_status", "/jwks", "", 503},
		{"jwks_json", "/jwks", "invalid-json", 200},
		{"jwks_oversize", "/jwks", strings.Repeat(" ", (1<<20)+1), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOAuthFixture(t, "client_secret_basic")
			f.service.client.Transport = logoutTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == tc.path {
					if tc.status == 0 {
						return nil, errors.New("temporary network failure")
					}
					return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			if err := f.service.BackchannelLogout(context.Background(), f.logoutToken(t, nil)); !errors.Is(err, ErrOAuthUnavailable) {
				t.Fatalf("temporary failure: %v", err)
			}
		})
	}
}

func TestBackchannelLogoutTransactionRollback(t *testing.T) {
	for _, table := range []string{"oauth_logouts", "oauth_sessions"} {
		t.Run(table, func(t *testing.T) {
			f := newOAuthFixture(t, "client_secret_basic")
			raw, _ := f.login(t)
			operation := "INSERT"
			if table == "oauth_sessions" {
				operation = "DELETE"
			}
			if err := f.db.Exec("CREATE TRIGGER fail_logout BEFORE " + operation + " ON " + table + " BEGIN SELECT RAISE(FAIL, 'disk full'); END").Error; err != nil {
				t.Fatal(err)
			}
			notice := f.logoutToken(t, nil)
			if err := f.service.BackchannelLogout(context.Background(), notice); !errors.Is(err, ErrOAuthUnavailable) {
				t.Fatalf("database failure: %v", err)
			}
			var count int64
			f.db.Model(&models.OAuthLogout{}).Count(&count)
			if count != 0 {
				t.Fatal("failed transaction persisted logout")
			}
			if _, err := f.service.Authenticate(context.Background(), raw); err != nil {
				t.Fatalf("rollback lost session: %v", err)
			}
			if err := f.db.Exec("DROP TRIGGER fail_logout").Error; err != nil {
				t.Fatal(err)
			}
			if err := f.service.BackchannelLogout(context.Background(), notice); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
			if _, err := f.service.Authenticate(context.Background(), raw); err == nil {
				t.Fatal("retry did not end session")
			}
		})
	}
}

func TestOAuthLoginRechecksIDExpiry(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	expiry := time.Unix(time.Now().Unix()+2, 0)
	f.p.modify = func(c jwt.MapClaims) { c["exp"] = expiry.Unix() }
	browser, query := f.begin(t)
	entered, resume := pauseOAuthResponse(t, f.service, "/userinfo")
	done := make(chan error, 1)
	go func() { _, _, err := f.service.CompleteLogin(context.Background(), browser, query); done <- err }()
	awaitOAuth(t, entered)
	<-time.After(time.Until(expiry) + time.Millisecond)
	resume()
	if err := awaitOAuth(t, done); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired ID token committed: %v", err)
	}
	var count int64
	f.db.Model(&models.OAuthSession{}).Count(&count)
	if count != 0 {
		t.Fatal("expired ID token created session")
	}
}

func TestOAuthLogoutCleanup(t *testing.T) {
	f := newOAuthFixture(t, "client_secret_basic")
	raw, _ := f.login(t)
	for _, row := range []models.OAuthLogout{
		{TargetKey: digest("expired"), ExpiresAt: time.Now().Add(-time.Hour)},
		{TargetKey: digest("active"), ExpiresAt: time.Now().Add(time.Hour)},
	} {
		if err := f.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.service.repo.Cleanup(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	var count int64
	f.db.Model(&models.OAuthLogout{}).Count(&count)
	if count != 1 {
		t.Fatal("cleanup did not retain only active logout records")
	}
	if _, err := f.service.Authenticate(context.Background(), raw); err != nil {
		t.Fatal("cleanup removed current session")
	}
	// 相关写入也必须清理到期记录。
	if err := f.db.Create(&models.OAuthLogout{TargetKey: digest("expired-again"), ExpiresAt: time.Now().Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	f.login(t)
	f.db.Model(&models.OAuthLogout{}).Count(&count)
	if count != 1 {
		t.Fatal("login did not clean expired logout records")
	}
	if err := f.db.Create(&models.OAuthLogout{TargetKey: digest("expired-on-logout"), ExpiresAt: time.Now().Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.service.BackchannelLogout(context.Background(), f.logoutToken(t, nil)); err != nil {
		t.Fatal(err)
	}
	f.db.Model(&models.OAuthLogout{}).Count(&count)
	if count != 2 {
		t.Fatal("logout did not clean expired records")
	}
}
