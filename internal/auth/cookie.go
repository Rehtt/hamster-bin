package auth

import (
	"net/http"

	"github.com/Rehtt/hamster-bin/internal/config"
)

func Cookie(cfg *config.Config, value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: CookieName, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: cfg.CookieSecure(), SameSite: http.SameSiteLaxMode}
}
