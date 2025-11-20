package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuthService 认证服务结构体
type AuthService struct {
	jwtSecret            string
	sessionKeyStore      sync.Map
	sessionKeyExpiration time.Duration
	stopChan             chan struct{}
}

// SessionKeyInfo SessionKey 信息结构
type SessionKeyInfo struct {
	SessionKey string
	ExpiresAt  time.Time
}

// JWTCustomClaims JWT 自定义声明
type JWTCustomClaims struct {
	OpenID         string `json:"openid"`
	DeviceID       string `json:"device_id"`
	SessionKeyHash string `json:"skh"`
	jwt.RegisteredClaims
}

// NewAuthService 创建认证服务实例
func NewAuthService(sessionKeyExpiration time.Duration) (*AuthService, error) {
	// 初始化 JWT Secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("生成 JWT Secret 失败: %v", err)
	}
	jwtSecret := base64.URLEncoding.EncodeToString(secretBytes)
	logger.Info("JWT Secret generated (will change after program restart)")

	service := &AuthService{
		jwtSecret:            jwtSecret,
		sessionKeyStore:      sync.Map{},
		sessionKeyExpiration: sessionKeyExpiration,
		stopChan:             make(chan struct{}),
	}

	// 启动统一的清理协程
	go service.startCleanupRoutine()

	return service, nil
}

// SaveSessionKey 保存 SessionKey
func (a *AuthService) SaveSessionKey(openID, sessionKey string) {
	a.sessionKeyStore.Store(openID, SessionKeyInfo{
		SessionKey: sessionKey,
		ExpiresAt:  time.Now().Add(a.sessionKeyExpiration),
	})
}

// GetSessionKey 获取 SessionKey
func (a *AuthService) GetSessionKey(openID string) (string, bool) {
	value, exists := a.sessionKeyStore.Load(openID)
	if !exists {
		return "", false
	}
	info, ok := value.(SessionKeyInfo)
	if !ok {
		return "", false
	}
	// 检查是否过期
	if time.Now().After(info.ExpiresAt) {
		a.sessionKeyStore.Delete(openID)
		return "", false
	}
	return info.SessionKey, true
}

// HashSessionKey 计算 SessionKey 的哈希值
func (a *AuthService) HashSessionKey(sessionKey string) string {
	hash := sha256.Sum256([]byte(sessionKey))
	return hex.EncodeToString(hash[:])
}

// GenToken 生成 Token（包含设备指纹和 SessionKey 哈希）
func (a *AuthService) GenToken(openID, deviceID, sessionKeyHash string) (string, error) {
	claims := JWTCustomClaims{
		OpenID:         openID,
		DeviceID:       deviceID,
		SessionKeyHash: sessionKeyHash,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.jwtSecret))
}

// ParseToken 解析 Token 并验证设备指纹和 SessionKey
func (a *AuthService) ParseToken(tokenStr string) (string, string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTCustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(a.jwtSecret), nil
	})
	if err != nil {
		return "", "", err
	}

	if claims, ok := token.Claims.(*JWTCustomClaims); ok && token.Valid {
		openID := claims.OpenID
		deviceID := claims.DeviceID

		// 验证 SessionKey 是否有效
		storedSessionKey, exists := a.GetSessionKey(openID)
		if !exists {
			return "", "", fmt.Errorf("session expired or invalidated")
		}

		// 验证 SessionKey 哈希是否匹配
		currentHash := a.HashSessionKey(storedSessionKey)
		if currentHash != claims.SessionKeyHash {
			return "", "", fmt.Errorf("session key mismatch - user may have logged in on another device")
		}

		return openID, deviceID, nil
	}
	return "", "", fmt.Errorf("invalid token")
}

// startCleanupRoutine 启动后台清理协程，定期清理过期的 session keys
func (a *AuthService) startCleanupRoutine() {
	// 清理间隔设为过期时间的 1/10，但最少 1 分钟，最多 1 小时
	cleanupInterval := a.sessionKeyExpiration / 10
	if cleanupInterval < time.Minute {
		cleanupInterval = time.Minute
	}
	if cleanupInterval > time.Hour {
		cleanupInterval = time.Hour
	}

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			expiredKeys := []string{}
			// 遍历所有 session keys，找出过期的
			a.sessionKeyStore.Range(func(key, value interface{}) bool {
				openID := key.(string)
				info, ok := value.(SessionKeyInfo)
				if !ok {
					// 无效的数据，直接删除
					a.sessionKeyStore.Delete(openID)
					return true
				}
				if now.After(info.ExpiresAt) {
					expiredKeys = append(expiredKeys, openID)
				}
				return true
			})
			// 删除过期的 keys
			for _, openID := range expiredKeys {
				a.sessionKeyStore.Delete(openID)
			}
			if len(expiredKeys) > 0 {
				logger.Infof("Cleaned up %d expired session keys", len(expiredKeys))
			}
		case <-a.stopChan:
			return
		}
	}
}

// Stop 停止认证服务（用于优雅关闭）
func (a *AuthService) Stop() {
	close(a.stopChan)
}

// AuthRequired 创建登录态保护中间件
func (a *AuthService) AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if h == "" || !strings.HasPrefix(h, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"msg": "missing token"})
		}
		openID, deviceID, err := a.ParseToken(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"msg": err.Error()})
		}
		c.Locals("openid", openID)
		c.Locals("device_id", deviceID)
		logger.Infof("auth required open_id[%s] device_id[%s]", openID, deviceID)
		return c.Next()
	}
}
