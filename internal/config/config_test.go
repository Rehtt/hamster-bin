package config

import "testing"

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
