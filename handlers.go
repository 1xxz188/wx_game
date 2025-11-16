package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// LoginReq 登录请求结构
type LoginReq struct {
	Code     string `json:"code"`
	DeviceID string `json:"device_id"` // 设备唯一标识
}

// AppServices 应用服务集合
type AppServices struct {
	AuthService   *AuthService
	WeChatService *WeChatService
	WSService     *WSService
	Config        *Config
}

// NewAppServices 创建应用服务实例
func NewAppServices(cfg *Config) (*AppServices, error) {
	// 创建认证服务
	authService, err := NewAuthService(cfg.GetSessionKeyExpiration())
	if err != nil {
		return nil, fmt.Errorf("创建认证服务失败: %v", err)
	}

	// 创建微信服务
	wechatService := NewWeChatService(cfg.WeChat.AppID, cfg.WeChat.AppSecret)

	// 创建 WebSocket 服务（传入章节索引存储）
	wsService := NewWSService(authService)

	return &AppServices{
		AuthService:   authService,
		WeChatService: wechatService,
		WSService:     wsService,
		Config:        cfg,
	}, nil
}

// LoginHandler 处理登录请求
func (app *AppServices) LoginHandler(c *fiber.Ctx) error {
	var req LoginReq
	clientIP := c.IP()

	if err := c.BodyParser(&req); err != nil || req.Code == "" {
		log.Printf("[登录失败] IP: %s, 原因: 请求参数无效, Code: %s", clientIP, req.Code)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"msg": "bad code"})
	}

	// DeviceID 为空时生成一个默认值（用于兼容旧客户端）
	if req.DeviceID == "" {
		req.DeviceID = "default-device"
	}

	var openID string
	var sessionKey string
	var loginSuccess bool

	// 开发模式支持
	if app.Config.App.DevMode && req.Code == "fake-code-for-test" {
		openID = "dev-openid-123"
		sessionKey = "dev-session-key-123"
		loginSuccess = true
		log.Printf("[登录成功] 开发模式 - IP: %s, DeviceID: %s, OpenID: %s", clientIP, req.DeviceID, openID)
	} else {
		session, err := app.WeChatService.Code2Session(req.Code)
		if err != nil {
			loginSuccess = false
			log.Printf("[登录失败] IP: %s, DeviceID: %s, 原因: 微信登录验证失败 - %v", clientIP, req.DeviceID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"msg": err.Error()})
		}
		openID = session.OpenID
		sessionKey = session.SessionKey
		loginSuccess = true
	}

	// 保存 SessionKey 到内存
	app.AuthService.SaveSessionKey(openID, sessionKey)

	// 计算 SessionKey 哈希
	sessionKeyHash := app.AuthService.HashSessionKey(sessionKey)

	// 生成包含设备指纹和 SessionKey 哈希的 JWT
	token, err := app.AuthService.GenToken(openID, req.DeviceID, sessionKeyHash)
	if err != nil {
		log.Printf("[登录失败] IP: %s, OpenID: %s, DeviceID: %s, 原因: Token 生成失败 - %v", clientIP, openID, req.DeviceID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"msg": "gen token failed"})
	}

	// 记录成功登录日志
	if loginSuccess {
		// 对 OpenID 进行部分脱敏处理（仅显示前4位和后4位）
		maskedOpenID := maskOpenID(openID)
		log.Printf("[登录成功] IP: %s, DeviceID: %s, OpenID: %s, 时间: %s",
			clientIP, req.DeviceID, maskedOpenID, time.Now().Format("2006-01-02 15:04:05"))
	}

	return c.JSON(fiber.Map{"token": token})
}

// maskOpenID 对 OpenID 进行脱敏处理（仅显示前4位和后4位）
func maskOpenID(openID string) string {
	if len(openID) <= 8 {
		return "****"
	}
	return openID[:4] + "****" + openID[len(openID)-4:]
}
