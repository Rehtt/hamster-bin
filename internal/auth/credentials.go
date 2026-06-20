package auth

import "crypto/subtle"

// CheckCredentials 使用恒定时间比较校验管理员凭据
func CheckCredentials(username, password, expectedUsername, expectedPassword string) bool {
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
	return usernameMatch && passwordMatch
}
