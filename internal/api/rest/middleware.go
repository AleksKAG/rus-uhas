package rest

import (
	"context"
	"net/http"
	"strings"

	"rus-uhas/internal/telemetry"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

// authMiddleware проверяет JWT токен
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем токен из заголовка
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}
		
		// Формат: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
			return
		}
		
		tokenString := parts[1]
		
		// Валидируем токен
		claims, err := s.auth.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusUnauthorized)
			return
		}
		
		// Добавляем пользователя в контекст
		ctx := context.WithValue(r.Context(), userContextKey, claims)
		ctx = telemetry.WithSurgeonID(ctx, claims.UserID)
		
		// Продолжаем обработку
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getCurrentUserFromClaims извлекает пользователя из контекста
func getCurrentUserFromClaims(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(userContextKey).(*Claims); ok {
		return claims
	}
	return nil
}
