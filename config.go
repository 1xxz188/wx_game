package main

import (
	"fmt"
	"os"
	"time"

	"github.com/donnie4w/go-logger/logger"
	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	App     AppConfig     `yaml:"app"`
	WeChat  WeChatConfig  `yaml:"wechat"`
	Session SessionConfig `yaml:"session"`
	Persist PersistConfig `yaml:"persist"`
	Mongo   MongoConfig   `yaml:"Mongo"`
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

// PersistConfig 定时落库配置
type PersistConfig struct {
	IntervalSeconds int `yaml:"interval_seconds"` // 定时间隔（秒）
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
	if c.Persist.IntervalSeconds <= 0 {
		return 60 * time.Second // 默认60秒
	}
	return time.Duration(c.Persist.IntervalSeconds) * time.Second
}
