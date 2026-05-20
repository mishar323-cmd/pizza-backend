package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type ctxKey int

const claimsKey ctxKey = 0

func RequireAdmin(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				writeJSONErr(w, http.StatusUnauthorized, "missing token")
				return
			}
			tokenStr := strings.TrimPrefix(h, "Bearer ")
			claims, err := ParseToken(secret, tokenStr)
			if err != nil {
				writeJSONErr(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func FromContext(ctx context.Context) *Claims {
	if c, ok := ctx.Value(claimsKey).(*Claims); ok {
		return c
	}
	return nil
}

func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
