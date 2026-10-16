package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const CookieName = "hamster_token"

var (
	ErrInvalidToken = errors.New("invalid token")
)

type Claims struct {
	Username   string `json:"username"`
	AuthMethod string `json:"auth_method,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	jwt.RegisteredClaims
}

// IssueOAuthToken 签发应用自己的会话引用，不携带提供方令牌。
func IssueOAuthToken(username, sessionID, secret string, expires time.Time) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Username: username, AuthMethod: "oauth", SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(expires)},
	}).SignedString([]byte(secret))
}

// IssueToken 签发 JWT
func IssueToken(username, secret string, expireHours int) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireHours) * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("签发 token 失败: %w", err)
	}
	return signed, nil
}

// ParseToken 解析并校验 JWT
func ParseToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.Username == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
