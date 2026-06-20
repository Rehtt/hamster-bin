package auth

import (
	"testing"
	"time"
)

func TestIssueAndParseToken(t *testing.T) {
	secret := "test-secret-key"
	username := "admin"

	token, err := IssueToken(username, secret, 1)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	claims, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.Username != username {
		t.Fatalf("username = %q, want %q", claims.Username, username)
	}
}

func TestParseTokenInvalidSecret(t *testing.T) {
	token, err := IssueToken("admin", "secret-a", 1)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	if _, err := ParseToken(token, "secret-b"); err == nil {
		t.Fatal("expected invalid token error")
	}
}

func TestParseTokenExpired(t *testing.T) {
	secret := "test-secret-key"
	token, err := IssueToken("admin", secret, -1)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	time.Sleep(time.Second)
	if _, err := ParseToken(token, secret); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestCheckCredentials(t *testing.T) {
	if !CheckCredentials("admin", "pass", "admin", "pass") {
		t.Fatal("expected valid credentials")
	}
	if CheckCredentials("admin", "wrong", "admin", "pass") {
		t.Fatal("expected invalid password")
	}
	if CheckCredentials("wrong", "pass", "admin", "pass") {
		t.Fatal("expected invalid username")
	}
}
