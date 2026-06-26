package rest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"rus-uhas/internal/telemetry"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
)

// User представляет пользователя системы
type User struct {
	ID         string
	BadgeID    string
	Name       string
	Role       string  // "surgeon", "nurse", "admin"
	Specialty  string
	PIN        string  // Хэш PIN-кода
	CreatedAt  time.Time
}

// AuthService управляет аутентификацией
type AuthService struct {
	jwtSecret    []byte
	tokenExpiry  time.Duration
	users        map[string]*User  // badge_id -> user
}

// NewAuthService создает новый сервис аутентификации
func NewAuthService(jwtSecret string, tokenExpiry time.Duration) *AuthService {
	// В реальности пользователи должны быть в БД
	users := map[string]*User{
		"BADGE001": {
			ID:        "user_1",
			BadgeID:   "BADGE001",
			Name:      "Иванов Иван Иванович",
			Role:      "surgeon",
			Specialty: "Нейрохирургия",
		},
		"BADGE002": {
			ID:        "user_2",
			BadgeID:   "BADGE002",
			Name:      "Петров Петр Петрович",
			Role:      "surgeon",
			Specialty: "Гепатология",
		},
		"BADGE003": {
			ID:        "user_3",
			BadgeID:   "BADGE003",
			Name:      "Сидорова Мария Петровна",
			Role:      "nurse",
			Specialty: "",
		},
	}
	
	return &AuthService{
		jwtSecret:   []byte(jwtSecret),
		tokenExpiry: tokenExpiry,
		users:       users,
	}
}

// Claims - JWT claims
type Claims struct {
	UserID    string `json:"user_id"`
	BadgeID   string `json:"badge_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// LoginResponse - ответ на логин
type LoginResponse struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      UserResponse `json:"user"`
}

// UserResponse - информация о пользователе
type UserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Specialty string `json:"specialty,omitempty"`
}

// Login выполняет аутентификацию по RFID badge
func (s *AuthService) Login(badgeID, pin string) (*LoginResponse, error) {
	user, ok := s.users[badgeID]
	if !ok {
		return nil, ErrInvalidCredentials
	}
	
	// Проверка PIN (если требуется)
	if user.PIN != "" && user.PIN != hashPIN(pin) {
		return nil, ErrInvalidCredentials
	}
	
	// Создаем JWT токен
	expiresAt := time.Now().Add(s.tokenExpiry)
	claims := &Claims{
		UserID:  user.ID,
		BadgeID: user.BadgeID,
		Role:    user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "rus-uhas",
			Subject:   user.ID,
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}
	
	return &LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
		User: UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Role:      user.Role,
			Specialty: user.Specialty,
		},
	}, nil
}

// ValidateToken проверяет и парсит JWT токен
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}
	
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	
	return claims, nil
}

// hashPIN хэширует PIN-код (в реальности использовать bcrypt)
func hashPIN(pin string) string {
	// Упрощенная версия для демонстрации
	// В реальности: bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	return pin
}

// GenerateJWTSecret генерирует случайный секрет для JWT
func GenerateJWTSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
