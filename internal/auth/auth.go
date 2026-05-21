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

// UserClaims represents a customer (phone-auth) session — distinct from the
// admin Claims above. `Kind` is always "user" so middleware can reject if
// a token of the wrong kind is presented at a kind-specific endpoint.
type UserClaims struct {
	UserID int64  `json:"uid"`
	Phone  string `json:"phone"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // always "user"
	jwt.RegisteredClaims
}

func IssueUserToken(secret []byte, c UserClaims) (string, error) {
	c.Kind = "user"
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(TokenLifetime))
	c.IssuedAt = jwt.NewNumericDate(time.Now())
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(secret)
}

func ParseUserToken(secret []byte, tokenStr string) (*UserClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*UserClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Kind != "user" {
		return nil, errors.New("not a user token")
	}
	return claims, nil
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
