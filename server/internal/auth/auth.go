package auth

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken = errors.New("invalid or tampered token")
	ErrExpiredToken = errors.New("token has expired")
	ErrRevokedToken = errors.New("token has been revoked")
)

type TokenManager struct {
	secretKey     []byte
	revokedTokens sync.Map // Map[tokenString]time.Time for invalidated tokens
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{
		secretKey: []byte(secret),
	}
}

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	TokenID  string `json:"jti"`
	jwt.RegisteredClaims
}

// HashPassword hashes plain password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash verifies password against bcrypt hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateTokens creates access and refresh tokens
func (tm *TokenManager) GenerateTokens(userID int64, username string) (string, string, string, error) {
	tokenID := fmt.Sprintf("%d_%d", userID, time.Now().UnixNano())

	accessClaims := Claims{
		UserID:   userID,
		Username: username,
		TokenID:  tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString(tm.secretKey)
	if err != nil {
		return "", "", "", err
	}

	refreshClaims := Claims{
		UserID:   userID,
		Username: username,
		TokenID:  tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString(tm.secretKey)
	if err != nil {
		return "", "", "", err
	}

	return accessStr, refreshStr, tokenID, nil
}

// RevokeToken invalidates a token or previous user session tokens
func (tm *TokenManager) RevokeToken(tokenStr string) {
	if tokenStr != "" {
		tm.revokedTokens.Store(tokenStr, time.Now())
	}
}

// ValidateToken parses and validates JWT token with strict signature method checks
func (tm *TokenManager) ValidateToken(tokenStr string) (*Claims, error) {
	if _, revoked := tm.revokedTokens.Load(tokenStr); revoked {
		return nil, ErrRevokedToken
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Enforce HS256 algorithm signature check (prevents alg: "none" exploit)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
