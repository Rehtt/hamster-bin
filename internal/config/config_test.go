package config

import "testing"

func TestMCPConfiguration(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("MCP_TOKEN", "")
	t.Setenv("MCP_ALLOWED_ORIGINS", "")
	if cfg := Load(); cfg.MCPToken != "" || len(cfg.MCPAllowedOrigins) != 0 {
		t.Fatalf("MCP should default to disabled: %+v", cfg.MCPAllowedOrigins)
	}
	t.Setenv("MCP_TOKEN", "secret")
	t.Setenv("MCP_ALLOWED_ORIGINS", " https://client.example, , http://localhost:3000 ")
	cfg := Load()
	if cfg.MCPToken != "secret" || len(cfg.MCPAllowedOrigins) != 2 || cfg.MCPAllowedOrigins[0] != "https://client.example" || cfg.MCPAllowedOrigins[1] != "http://localhost:3000" {
		t.Fatal("MCP environment configuration was not loaded correctly")
	}
}

func TestValidateAllowsDefaultSQLite(t *testing.T) {
	cfg := &Config{DBDriver: "sqlite", DBPath: defaultDBPath}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresDSNForExternalDatabases(t *testing.T) {
	for _, driver := range []string{"mysql", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := &Config{DBDriver: driver}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() error = nil, want DB_DSN error")
			}
		})
	}
}

func TestNormalizeDBDriver(t *testing.T) {
	tests := map[string]string{
		"":           "sqlite",
		" sqlite3 ":  "sqlite",
		"MySQL":      "mysql",
		"postgresql": "postgres",
		"Postgres":   "postgres",
	}

	for input, want := range tests {
		if got := normalizeDBDriver(input); got != want {
			t.Fatalf("normalizeDBDriver(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDatabaseDisplay(t *testing.T) {
	sqliteCfg := &Config{DBDriver: "sqlite", DBPath: "/tmp/inventory.db"}
	if got := sqliteCfg.DatabaseDisplay(); got != "/tmp/inventory.db" {
		t.Fatalf("SQLite DatabaseDisplay() = %q", got)
	}

	externalCfg := &Config{
		DBDriver: "mysql",
		DBDSN:    "user:password@tcp(127.0.0.1:3306)/hamster_bin",
	}
	if got := externalCfg.DatabaseDisplay(); got != "external" {
		t.Fatalf("external DatabaseDisplay() = %q, want external", got)
	}
}

func TestTaskConfigurationDefaultsAndOverrides(t *testing.T) {
	for _, test := range []struct {
		workers, timeout         string
		wantWorkers, wantTimeout int
	}{
		{"", "", 2, 120}, {"3", "45", 3, 45}, {"0", "-1", 2, 120}, {"bad", "bad", 2, 120},
	} {
		t.Setenv("TASK_WORKERS", test.workers)
		t.Setenv("TASK_TIMEOUT_SECONDS", test.timeout)
		if got := positiveEnv("TASK_WORKERS", 2); got != test.wantWorkers {
			t.Fatalf("workers=%d", got)
		}
		if got := positiveEnv("TASK_TIMEOUT_SECONDS", 120); got != test.wantTimeout {
			t.Fatalf("timeout=%d", got)
		}
	}
}

func TestOAuthConfigValidation(t *testing.T) {
	valid := Config{DBDriver: "sqlite", OAuthEnabled: true, OAuthIssuer: "https://oauth.home.rehtt.com", OAuthClientID: "client", OAuthClientSecret: "secret", OAuthRedirectURL: "https://inventory.example.com/api/v1/auth/oauth/callback", OAuthAuthMethod: "client_secret_basic", JWTSecret: "jwt-secret"}
	for _, tc := range []struct {
		name   string
		change func(*Config)
		ok     bool
	}{
		{"valid", func(*Config) {}, true},
		{"post", func(c *Config) { c.OAuthAuthMethod = "client_secret_post" }, true},
		{"local", func(c *Config) {
			c.OAuthIssuer = "http://127.0.0.1:9000"
			c.OAuthRedirectURL = "http://localhost:5173/api/v1/auth/oauth/callback"
		}, true},
		{"ipv6", func(c *Config) { c.OAuthRedirectURL = "http://[::1]:5173/api/v1/auth/oauth/callback" }, true},
		{"secret", func(c *Config) { c.OAuthClientSecret = "" }, false},
		{"client", func(c *Config) { c.OAuthClientID = "" }, false},
		{"jwt", func(c *Config) { c.JWTSecret = "" }, false},
		{"method", func(c *Config) { c.OAuthAuthMethod = "auto" }, false},
		{"issuer_slash", func(c *Config) { c.OAuthIssuer += "/" }, false},
		{"issuer_query", func(c *Config) { c.OAuthIssuer += "?x=1" }, false},
		{"issuer_missing", func(c *Config) { c.OAuthIssuer = "" }, false},
		{"issuer_http", func(c *Config) { c.OAuthIssuer = "http://oauth.example.com" }, false},
		{"callback_http", func(c *Config) { c.OAuthRedirectURL = "http://inventory.example.com/api/v1/auth/oauth/callback" }, false},
		{"callback_credentials", func(c *Config) { c.OAuthRedirectURL = "https://u:p@inventory.example.com/api/v1/auth/oauth/callback" }, false},
		{"callback_fragment", func(c *Config) { c.OAuthRedirectURL += "#x" }, false},
		{"callback_query", func(c *Config) { c.OAuthRedirectURL += "?code=x" }, false},
		{"callback_path", func(c *Config) { c.OAuthRedirectURL += "/" }, false},
		{"callback_missing", func(c *Config) { c.OAuthRedirectURL = "" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.change(&cfg)
			if err := cfg.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate()=%v, want valid=%v", err, tc.ok)
			}
		})
	}
	if !valid.IsAuthEnabled() || valid.IsPasswordAuthEnabled() || !valid.CookieSecure() || valid.IsHTTPS() {
		t.Fatal("OAuth-only/proxy configuration incorrect")
	}
	valid.OAuthRedirectURL = "HTTPS://inventory.example.com/api/v1/auth/oauth/callback"
	if err := valid.Validate(); err != nil || !valid.CookieSecure() {
		t.Fatal("HTTPS scheme case disabled secure cookie")
	}
	valid.OAuthEnabled = false
	if valid.IsAuthEnabled() || valid.CookieSecure() {
		t.Fatal("OAuth did not disable")
	}
}

func TestOAuthEnvironmentDefaultsAndEnabled(t *testing.T) {
	for _, key := range []string{"ADMIN_USERNAME", "ADMIN_PASSWORD", "JWT_SECRET", "OAUTH_ENABLED", "OAUTH_ISSUER", "OAUTH_CLIENT_ID", "OAUTH_CLIENT_SECRET", "OAUTH_REDIRECT_URL", "OAUTH_AUTH_METHOD"} {
		t.Setenv(key, "")
	}
	t.Setenv("DB_DRIVER", "sqlite")
	cfg := Load()
	if cfg.OAuthEnabled || cfg.OAuthIssuer != "https://oauth.home.rehtt.com" || cfg.OAuthAuthMethod != "client_secret_basic" {
		t.Fatal("unexpected OAuth defaults")
	}
	t.Setenv("OAUTH_ENABLED", "true")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("OAUTH_CLIENT_ID", "client")
	t.Setenv("OAUTH_CLIENT_SECRET", "secret")
	t.Setenv("OAUTH_REDIRECT_URL", "https://inventory.example.com/api/v1/auth/oauth/callback")
	if cfg = Load(); !cfg.IsAuthEnabled() || !cfg.OAuthEnabled {
		t.Fatal("OAuth environment ignored")
	}
	t.Setenv("OAUTH_ENABLED", "typo")
	defer func() {
		if recover() == nil {
			t.Error("invalid OAUTH_ENABLED silently disabled auth")
		}
	}()
	Load()
}
