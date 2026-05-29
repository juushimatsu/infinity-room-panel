package auth

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthConfig struct {
	HashedPassword string `json:"hashed_password"`
	JWTSecret      string `json:"jwt_secret"`
}

func GeneratePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateToken(jwtSecret string) (string, error) {
	claims := jwt.MapClaims{
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func ValidateToken(tokenStr, jwtSecret string) (bool, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return false, err
	}
	return token.Valid, nil
}

func SaveAuthConfig(configDir string, hashedPassword, jwtSecret string) error {
	cfg := AuthConfig{
		HashedPassword: hashedPassword,
		JWTSecret:      jwtSecret,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "auth.json"), data, 0600)
}

func LoadAuthConfig(configDir string) (*AuthConfig, error) {
	data, err := os.ReadFile(filepath.Join(configDir, "auth.json"))
	if err != nil {
		return nil, err
	}
	var cfg AuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func generateSecret(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func InitAuth(configDir string, isElectron bool) (*AuthConfig, error) {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	cfg, err := LoadAuthConfig(configDir)
	if err == nil && cfg != nil {
		return cfg, nil
	}

	// First run — generate new config
	if isElectron {
		// No auth needed for Electron
		secret := generateSecret(32)
		cfg = &AuthConfig{
			HashedPassword: "",
			JWTSecret:      secret,
		}
	} else {
		password := GeneratePassword(16)
		hash, err := HashPassword(password)
		if err != nil {
			return nil, err
		}
		secret := generateSecret(32)
		cfg = &AuthConfig{
			HashedPassword: hash,
			JWTSecret:      secret,
		}
		fmt.Printf("\n=== Сгенерированный пароль для входа: %s ===\n\n", password)
	}

	if err := SaveAuthConfig(configDir, cfg.HashedPassword, cfg.JWTSecret); err != nil {
		return nil, err
	}
	return cfg, nil
}
