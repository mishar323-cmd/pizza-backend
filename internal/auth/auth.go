package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const TokenLifetime = 7 * 24 * time.Hour

type Claims struct {
	AdminID int64  `json:"aid"`
	Login   string `json:"login"`
	Role    string `json:"role"`
	Name    string `json:"name"`
	jwt.RegisteredClaims
}

func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func IssueToken(secret []byte, c Claims) (string, error) {
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(TokenLifetime))
	c.IssuedAt = jwt.NewNumericDate(time.Now())
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(secret)
}

func ParseToken(secret []byte, tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
