package router

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/auth"
	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOAuthHTTPFlowAndPasswordCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	previous := gin.DefaultWriter
	gin.DefaultWriter = &logs
	defer func() { gin.DefaultWriter = previous }()
	previousLog := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousLog)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var provider *httptest.Server
	var revoked atomic.Bool
	var infoCalls atomic.Int32
	nonce := make(chan string, 4)
	challenge := make(chan string, 4)
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		send := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			send(map[string]any{"issuer": provider.URL, "authorization_endpoint": provider.URL + "/authorize", "token_endpoint": provider.URL + "/token", "userinfo_endpoint": provider.URL + "/userinfo", "jwks_uri": provider.URL + "/jwks", "revocation_endpoint": provider.URL + "/revoke", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			send(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "key", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/token":
			if r.ParseForm() != nil {
				w.WriteHeader(400)
				return
			}
			n, c := <-nonce, <-challenge
			if oauth2.S256ChallengeFromVerifier(r.Form.Get("code_verifier")) != c {
				w.WriteHeader(400)
				return
			}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": provider.URL, "sub": "user-1", "aud": "client", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "nonce": n})
			token.Header["kid"] = "key"
			signed, _ := token.SignedString(key)
			send(map[string]any{"access_token": "private-access", "refresh_token": "private-refresh", "id_token": signed, "token_type": "Bearer", "expires_in": 900})
		case "/userinfo":
			infoCalls.Add(1)
			if revoked.Load() || r.Header.Get("Authorization") != "Bearer private-access" {
				w.WriteHeader(401)
				return
			}
			send(map[string]any{"sub": "user-1", "preferred_username": "alice"})
		case "/revoke":
			revoked.Store(true)
			send(map[string]any{})
		default:
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "router.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.OAuthIdentity{}, &models.OAuthSession{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: "password", JWTSecret: "test-secret", JWTExpireHours: 168, OAuthEnabled: true, OAuthIssuer: provider.URL, OAuthClientID: "client", OAuthClientSecret: "private-client-secret", OAuthAuthMethod: "client_secret_basic", OAuthRedirectURL: "http://127.0.0.1:5173/api/v1/auth/oauth/callback"}
	engine := Setup(db, parser.NewParserManager(), cfg, nil)
	call := func(method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}
	cookie := func(w *httptest.ResponseRecorder, name string) *http.Cookie {
		t.Helper()
		for _, c := range w.Result().Cookies() {
			if c.Name == name {
				return c
			}
		}
		t.Fatalf("missing cookie %s", name)
		return nil
	}
	for _, tc := range []struct{ password, oauth bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		cfg.AdminPassword = ""
		if tc.password {
			cfg.AdminPassword = "password"
		}
		cfg.OAuthEnabled = tc.oauth
		w := call("GET", "/api/v1/auth/config", "")
		var response struct {
			Data map[string]bool `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Data["password_enabled"] != tc.password || response.Data["oauth_enabled"] != tc.oauth || response.Data["auth_enabled"] != (tc.password || tc.oauth) {
			t.Fatal("incorrect advertised login methods")
		}
		if w := call("GET", "/api/v1/platforms", ""); (w.Code == 200) == (tc.password || tc.oauth) {
			t.Fatal("incorrect protection")
		}
	}
	cfg.AdminPassword = "password"
	cfg.OAuthEnabled = true
	begin := call("GET", "/api/v1/auth/oauth/login", "")
	location, _ := url.Parse(begin.Header().Get("Location"))
	q := location.Query()
	nonce <- q.Get("nonce")
	challenge <- q.Get("code_challenge")
	flowCookie := cookie(begin, auth.FlowCookieName)
	if begin.Code != 302 || !flowCookie.HttpOnly || flowCookie.SameSite != http.SameSiteLaxMode || flowCookie.MaxAge != 600 {
		t.Fatal("invalid flow cookie")
	}
	callbackPath := "/api/v1/auth/oauth/callback?code=private-code&state=" + url.QueryEscape(q.Get("state"))
	callback := call("GET", callbackPath, "", flowCookie)
	if callback.Code != 302 || callback.Header().Get("Location") != "/" || callback.Header().Get("Cache-Control") != "no-store" || callback.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("callback failed: %d %s", callback.Code, callback.Header().Get("Location"))
	}
	sessionCookie := cookie(callback, auth.CookieName)
	if !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("invalid session cookie")
	}
	if flow := cookie(callback, auth.FlowCookieName); flow.MaxAge != -1 {
		t.Fatal("flow cookie not cleared")
	}
	me := call("GET", "/api/v1/auth/me", "", sessionCookie)
	if me.Code != 200 || !strings.Contains(me.Body.String(), `"auth_method":"oauth"`) || !strings.Contains(me.Body.String(), `"username":"alice"`) {
		t.Fatalf("me: %s", me.Body.String())
	}
	if w := call("GET", "/api/v1/platforms", "", sessionCookie); w.Code != 200 {
		t.Fatal("protected route denied")
	}
	if infoCalls.Load() != 3 {
		t.Fatal("protected requests skipped online check")
	}
	// 密码登录替换 OAuth 会话时，同时清理并撤销旧会话。
	password := call("POST", "/api/v1/auth/login", `{"username":"admin","password":"password"}`, sessionCookie)
	if password.Code != 200 || !revoked.Load() {
		t.Fatal("password login did not replace OAuth session")
	}
	adminCookie := cookie(password, auth.CookieName)
	if call("GET", "/api/v1/auth/me", "", sessionCookie).Code != 401 {
		t.Fatal("replaced OAuth session accepted")
	}
	if call("GET", "/api/v1/platforms", "", adminCookie).Code != 200 {
		t.Fatal("password session incorrectly uses OAuth online check")
	}
	cfg.AdminPassword = ""
	if call("POST", "/api/v1/auth/login", `{"username":"admin","password":"password"}`).Code != 400 || call("GET", "/api/v1/platforms", "", adminCookie).Code != 401 {
		t.Fatal("password credentials bypass OAuth-only mode")
	}
	cfg.OAuthRedirectURL = "https://inventory.example.com/api/v1/auth/oauth/callback"
	begin = call("GET", "/api/v1/auth/oauth/login", "")
	if !cookie(begin, auth.FlowCookieName).Secure {
		t.Fatal("reverse proxy HTTPS cookie not secure")
	}
	bad := call("GET", callbackPath, "", flowCookie)
	if !strings.HasPrefix(bad.Header().Get("Location"), "/login?oauth_error=") {
		t.Fatal("replayed callback accepted")
	}
	cfg.OAuthEnabled = false
	if call("GET", "/api/v1/auth/oauth/login", "").Code != 404 {
		t.Fatal("disabled OAuth endpoint active")
	}
	// 即使发生 panic，日志也不能输出完整回调查询、Cookie、Authorization 或 panic 值。
	engine.GET("/panic", func(*gin.Context) { panic("private-panic") })
	req := httptest.NewRequest("GET", "/panic?code=private-code", nil)
	req.Header.Set("Cookie", "private-cookie")
	req.Header.Set("Authorization", "private-authorization")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatal("panic was not recovered")
	}
	for _, secret := range []string{"private-client-secret", "private-access", "private-refresh", "private-code", "private-cookie", "private-authorization", "private-panic"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("log leaked %s", secret)
		}
	}
	// 本地退出不依赖提供方可用性。
	if w := call("POST", "/api/v1/auth/logout", "", adminCookie); w.Code != 200 || cookie(w, auth.CookieName).MaxAge != -1 {
		t.Fatal("logout failed")
	}
}
