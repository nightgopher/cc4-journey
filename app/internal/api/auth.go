package api

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxRole
)

func withUser(ctx context.Context, userID int, role string) context.Context {
	ctx = context.WithValue(ctx, ctxUserID, userID)
	return context.WithValue(ctx, ctxRole, role)
}

func mustUser(ctx context.Context) (int, string) {
	uid, _ := ctx.Value(ctxUserID).(int)
	role, _ := ctx.Value(ctxRole).(string)
	return uid, role
}

// signToken 对齐存量 generateToken：HS256，payload {userId}，7 天有效。
func signToken(userID int, secret []byte) (string, error) {
	claims := AuthClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(timeNow().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(timeNow()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func timeNow() time.Time { return time.Now() }

func fmt2(format string, a ...any) string { return fmt.Sprintf(format, a...) }

func checkPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil ||
		subtle.ConstantTimeCompare([]byte(password), []byte(hash)) == 1 // 兼容存量明文测试账号
}

func jwtSecretFromEnv() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "your_super_secret_jwt_key_here_change_in_production" // 与存量 docker-compose 默认值一致
}
