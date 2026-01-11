package main

import (
	"context"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
	"wx_game/cfg"
	"wx_game/fw/persistence"
	"wx_game/fw/persistence/mongoop"
	"wx_game/role"
	"wx_game/watermelon"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	defer func() {
		if rec := recover(); rec != nil {
			stackTrace := debug.Stack()
			stackTraceAsRawStringLiteral := strconv.Quote(string(stackTrace))
			logger.Errorf("rec: %v, stackTrace: %v", rec, stackTraceAsRawStringLiteral)
		}
	}()

	// 加载配置（先于日志初始化，使用临时控制台日志）
	config, err := LoadConfig("config.yaml")
	if err != nil {
		// 配置加载失败时使用默认日志设置
		logger.Errorf("%v", err)
		os.Exit(1)
	}

	// 根据配置初始化日志
	logger.SetOption(&logger.Option{
		Level:     config.GetLogLevel(),
		Formatter: "[{time}]	{level}	[{file}]	{message}\n", // 日志输出
		// 设置格式：包含日期、时间和毫秒
		Format: logger.FORMAT_LEVELFLAG | logger.FORMAT_SHORTFILENAME | logger.FORMAT_DATE | logger.FORMAT_TIME | logger.FORMAT_MICROSECONDS,
		FileOption: &logger.FileMixedMode{
			Filename:   config.GetLogPath(),
			Maxsize:    config.GetLogMaxSize(),
			Timemode:   logger.MODE_DAY,
			Maxbuckup:  config.GetLogMaxFiles(),
			IsCompress: false, // 不压缩，方便直接 tail
		},
		// 设置自定义时间格式：2025-10-23 10:17:10.027
		AttrFormat: &logger.AttrFormat{
			SetTimeFmt: func() (string, string, string) {
				currentTime := time.Now().Format("2006-01-02 15:04:05.000")
				return currentTime, "", ""
			},
		},
		Console: config.GetLogConsole(),
	})

	logger.Infof("Log configuration loaded: level=%s, path=%s", config.Log.Level, config.Log.Path)

	err = cfg.Init()
	if err != nil {
		logger.Fatal(err)
		return
	}

	roleMgr := role.New()

	// 初始化 MongoDB 客户端（必须成功）
	mongoCfg := mongoop.Conf{
		Url:           config.Mongo.Url,
		ConnTimeout:   config.Mongo.ConnTimeout,
		AuthMechanism: config.Mongo.AuthMechanism,
		User:          config.Mongo.User,
		Password:      config.Mongo.Password,
		MaxPoolSize:   config.Mongo.MaxPoolSize,
		IsAuthSource:  config.Mongo.IsAuthSource,
		Database:      config.Mongo.Database,
		FlushDbSec:    config.Mongo.FlushDbSec,
	}
	mongoClient, err := mongoop.NewMongoClientWithConf(mongoCfg)
	if err != nil {
		logger.Errorf("Failed to initialize MongoDB client: %v", err)
		os.Exit(1)
	}
	logger.Info("MongoDB client initialized successfully")

	// 从MongoDB加载userIdMap和lockNextId到内存（必须成功）
	err = roleMgr.LoadFromMongo(mongoClient)
	if err != nil {
		logger.Errorf("Failed to load userIdMap and lockNextId from MongoDB: %v", err)
		os.Exit(1)
	}
	logger.Info("userIdMap and lockNextId loaded from MongoDB successfully")

	// 创建定时落库管理器
	persistInterval := config.GetPersistInterval()
	persistMgr := persistence.NewPersistManager(mongoClient, persistInterval)

	// 注册需要保存的数据
	roleMgr.RegisterPersistFunc(persistMgr)

	// 启动定时落库
	persistMgr.Start()

	// 设置程序关闭时的清理函数
	defer func() {
		logger.Info("Application is shutting down, performing cleanup...")
		persistMgr.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.DisConnect(ctx); err != nil {
			logger.Errorf("Failed to disconnect MongoDB: %v", err)
		} else {
			logger.Info("MongoDB connection closed")
		}
	}()

	// 创建应用服务
	appServices, err := NewAppServices(config, roleMgr)
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}

	// 创建 Fiber 应用
	app := fiber.New(fiber.Config{BodyLimit: 200 << 20})

	// WebSocket 接口（需要在中间件之前注册，避免中间件干扰 WebSocket 握手）
	app.Get("/ws", appServices.WSService.Handler())

	// 安全响应头中间件
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// 仅在 HTTPS 模式下设置 HSTS
		if !config.App.DevMode && config.App.TLS.CertFile != "" {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		return c.Next()
	})

	// 速率限制中间件（排除 WebSocket）
	app.Use(limiter.New(limiter.Config{
		Max:        20,              // 每个 IP 每分钟最多 20 次请求
		Expiration: 1 * time.Minute, // 时间窗口为 1 分钟
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"msg": "请求过于频繁，请稍后再试",
			})
		},
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
		Next: func(c *fiber.Ctx) bool {
			// 排除 WebSocket 路径
			return c.Path() == "/ws"
		},
	}))

	// 登录接口特殊速率限制（更严格）
	loginLimiter := limiter.New(limiter.Config{
		Max:        5, // 每个 IP 每分钟最多 5 次登录尝试
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			logger.Infof("Login rate limit triggered - IP: %s", c.IP())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"msg": "登录请求过于频繁，请稍后再试",
			})
		},
		Next: func(c *fiber.Ctx) bool {
			// 排除本机IP，不进行速率限制
			ip := net.ParseIP(c.IP())
			if ip != nil {
				// 检查是否是本机IP: 127.0.0.1 或 ::1
				if ip.IsLoopback() {
					return true // 跳过速率限制
				}
			}
			return false // 继续速率限制检查
		},
	})

	// 日志中间件
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format: "${time} ${method} ${path} - ${status}\n",
		// 排除 WebSocket 路径，避免干扰握手
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/ws"
		},
	}))

	// 登录接口（应用特殊速率限制）
	app.Post("/api/login", loginLimiter, appServices.LoginHandler)

	// 需要登录态的接口
	//protected := app.Group("/", appServices.AuthService.AuthRequired())
	//protected.Post("/upload", appServices.UploadHandler)

	// 启动服务器
	port := config.App.Port
	if port == 0 {
		port = 8080
	}

	wMgr := watermelon.New()
	wMgr.Init(appServices.WSService.registry, roleMgr)

	// 设置信号处理，优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 在单独的 goroutine 中启动服务器
	serverErrChan := make(chan error, 1)
	go func() {
		// 根据配置决定使用 HTTP 还是 HTTPS
		if !config.App.DevMode && config.App.TLS.CertFile != "" && config.App.TLS.KeyFile != "" {
			// 生产环境使用 HTTPS
			logger.Infof("Starting HTTPS server on port: %d", port)
			logger.Infof("Certificate file: %s", config.App.TLS.CertFile)
			logger.Infof("Private key file: %s", config.App.TLS.KeyFile)
			if err := app.ListenTLS(":"+strconv.Itoa(port), config.App.TLS.CertFile, config.App.TLS.KeyFile); err != nil {
				serverErrChan <- err
			}
		} else {
			// 开发环境使用 HTTP
			if config.App.DevMode {
				logger.Infof("Dev mode: Starting HTTP server on port: %d", port)
			} else {
				logger.Info("Warning: Production environment not configured with TLS, using HTTP (insecure!)")
				logger.Infof("Starting HTTP server on port: %d", port)
			}
			if err := app.Listen(":" + strconv.Itoa(port)); err != nil {
				serverErrChan <- err
			}
		}
	}()

	// 等待信号或服务器错误
	select {
	case sig := <-sigChan:
		logger.Infof("Received shutdown signal: %v, starting graceful shutdown...", sig)
		// 关闭服务器
		if err := app.Shutdown(); err != nil {
			logger.Errorf("Failed to shutdown server: %v", err)
		}
	case err := <-serverErrChan:
		logger.Errorf("Server startup failed: %v", err)
		os.Exit(1)
	}
}
