package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
)

type oauthTokens struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
	Nonce   string `json:"nonce"`
}

func randomValue() string { return base64.RawURLEncoding.EncodeToString(randomBytes(32)) }

func randomBytes(n int) []byte {
	b := make([]byte, n)
	// crypto/rand.Read 在系统随机源故障时终止程序，不会返回不安全的随机数。
	_, _ = rand.Read(b)
	return b
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func identityKey(issuer, subject string) string {
	encoded, _ := json.Marshal([2]string{issuer, subject})
	return digest(string(encoded))
}

func targetKey(issuer, clientID, subject, sid string) string {
	encoded, _ := json.Marshal([4]string{issuer, clientID, subject, sid})
	return digest(string(encoded))
}

func tokenCipher(secret string) (cipher.AEAD, error) {
	key, err := hkdf.Key(sha256.New, []byte(secret), nil, "hamster-bin/oauth-tokens/v1", 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptTokens(secret, sessionID string, tokens oauthTokens) ([]byte, error) {
	aead, err := tokenCipher(secret)
	if err != nil {
		return nil, err
	}
	plain, err := json.Marshal(tokens)
	if err != nil {
		return nil, err
	}
	nonce := randomBytes(aead.NonceSize())
	return aead.Seal(nonce, nonce, plain, []byte(sessionID)), nil
}

func decryptTokens(secret, sessionID string, encrypted []byte) (oauthTokens, error) {
	var tokens oauthTokens
	aead, err := tokenCipher(secret)
	if err != nil {
		return tokens, err
	}
	if len(encrypted) < aead.NonceSize() {
		return tokens, ErrInvalidToken
	}
	plain, err := aead.Open(nil, encrypted[:aead.NonceSize()], encrypted[aead.NonceSize():], []byte(sessionID))
	if err != nil {
		return tokens, ErrInvalidToken
	}
	err = json.Unmarshal(plain, &tokens)
	return tokens, err
}
