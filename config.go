package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/donnie4w/go-logger/logger"
	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	App     AppConfig     `yaml:"app"`
	Log     LogConfig     `yaml:"log"`
	WeChat  WeChatConfig  `yaml:"wechat"`
	Session SessionConfig `yaml:"session"`
	Mongo   MongoConfig   `yaml:"Mongo"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `yaml:"level"`     // 日志等级: debug, info, warn, error, fatal, all
	Path     string `yaml:"path"`      // 日志文件路径
	MaxSize  int    `yaml:"max_size"`  // 单个日志文件最大大小(MB)
	MaxFiles int    `yaml:"max_files"` // 最多保留的日志文件数量
	Console  bool   `yaml:"console"`   // 是否输出到控制台
}

// MongoConfig MongoDB 数据库配置
type MongoConfig struct {
	Url           string `yaml:"url"`
	ConnTimeout   string `yaml:"conntimeout"`
	AuthMechanism string `yaml:"authmechanism"`
	User          string `yaml:"user"`
	Password      string `yaml:"passwd"`
	MaxPoolSize   uint64 `yaml:"maxpoolsize"`
	IsAuthSource  bool   `yaml:"isauthsource"`
	Database      string `yaml:"dbbase"`
	FlushDbSec    int32  `yaml:"flushdbsec"`
}

// AppConfig 应用配置
type AppConfig struct {
	Port    int       `yaml:"port"`
	DevMode bool      `yaml:"dev_mode"`
	TLS     TLSConfig `yaml:"tls,omitempty"`
}

// TLSConfig TLS/HTTPS 配置
type TLSConfig struct {
	CertFile string `yaml:"cert_file"` // 证书文件路径
	KeyFile  string `yaml:"key_file"`  // 私钥文件路径
}

// WeChatConfig 微信配置
type WeChatConfig struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

// SessionConfig Session 配置
type SessionConfig struct {
	KeyExpirationHours int `yaml:"key_expiration_hours"`
}

// LoadConfig 从 config.yaml 加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证必要配置
	/*if config.MinIO.Endpoint == "" {
		return nil, fmt.Errorf("minio.endpoint 不能为空")
	}*/

	logger.Infof("Configuration loaded successfully: %s", path)
	return &config, nil
}

// GetSessionKeyExpiration 获取 SessionKey 过期时间
func (c *Config) GetSessionKeyExpiration() time.Duration {
	if c.Session.KeyExpirationHours <= 0 {
		return 30 * 24 * time.Hour // 默认30天
	}
	return time.Duration(c.Session.KeyExpirationHours) * time.Hour
}

// GetPersistInterval 获取定时落库间隔
func (c *Config) GetPersistInterval() time.Duration {
	if c.Mongo.FlushDbSec <= 0 {
		return 60 * time.Second // 默认60秒
	}
	return time.Duration(c.Mongo.FlushDbSec) * time.Second
}

// GetLogLevel 获取日志等级
func (c *Config) GetLogLevel() logger.LEVELTYPE {
	switch strings.ToLower(c.Log.Level) {
	case "debug":
		return logger.LEVEL_DEBUG
	case "info":
		return logger.LEVEL_INFO
	case "warn":
		return logger.LEVEL_WARN
	case "error":
		return logger.LEVEL_ERROR
	case "fatal":
		return logger.LEVEL_FATAL
	case "all":
		return logger.LEVEL_ALL
	default:
		return logger.LEVEL_INFO // 默认 info 级别
	}
}

// GetLogPath 获取日志文件路径
func (c *Config) GetLogPath() string {
	if c.Log.Path == "" {
		return "app.log" // 默认日志文件名
	}
	return c.Log.Path
}

// GetLogMaxSize 获取日志文件最大大小(字节)
func (c *Config) GetLogMaxSize() int64 {
	if c.Log.MaxSize <= 0 {
		return 10 << 20 // 默认 10 MB
	}
	return int64(c.Log.MaxSize) << 20
}

// GetLogMaxFiles 获取最大日志文件数量
func (c *Config) GetLogMaxFiles() int {
	if c.Log.MaxFiles <= 0 {
		return 10 // 默认 10 个
	}
	return c.Log.MaxFiles
}

// GetLogConsole 获取是否输出到控制台
func (c *Config) GetLogConsole() bool {
	return c.Log.Console
}
