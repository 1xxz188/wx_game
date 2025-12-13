package main

import (
	"fmt"
	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/fiber/v2"
	"wx_game/role"
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
func NewAppServices(cfg *Config, roleMgr *role.Mgr) (*AppServices, error) {
	// 创建认证服务
	authService, err := NewAuthService(cfg.GetSessionKeyExpiration())
	if err != nil {
		return nil, fmt.Errorf("创建认证服务失败: %v", err)
	}

	// 创建微信服务
	wechatService := NewWeChatService(cfg.WeChat.AppID, cfg.WeChat.AppSecret)

	// 创建 WebSocket 服务（传入章节索引存储）
	wsService := NewWSService(authService, roleMgr)

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
		logger.Errorf("[Login failed] IP: %s, reason: invalid request parameters, Code: %s", clientIP, req.Code)
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
	if /*app.Config.App.DevMode && */ req.Code == "fake-code-for-test" {
		openID = "dev-openid-123"
		sessionKey = "dev-session-key-123"
		loginSuccess = true
		//logger.Infof("[Login successful] Dev mode - IP: %s, DeviceID: %s, OpenID: %s", clientIP, req.DeviceID, openID)
	} else {
		session, err := app.WeChatService.Code2Session(req.Code)
		if err != nil {
			loginSuccess = false
			logger.Errorf("[Login failed] IP: %s, DeviceID: %s, reason: WeChat login verification failed - %v", clientIP, req.DeviceID, err)
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
		logger.Errorf("[http Login failed] IP[%s] OpenID[%s] DeviceID[%s] reason Token generation failed - %v", clientIP, openID, req.DeviceID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"msg": "gen token failed"})
	}

	// 记录成功登录日志
	if loginSuccess {
		// 对 OpenID 进行部分脱敏处理（仅显示前4位和后4位）
		//maskedOpenID := maskOpenID(openID)
		logger.Infof("open_id[%s] [http Login ok] IP[%s] DeviceID[%s]", openID, clientIP, req.DeviceID)
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
