package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	App     AppConfig     `yaml:"app"`
	WeChat  WeChatConfig  `yaml:"wechat"`
	Session SessionConfig `yaml:"session"`
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

	log.Printf("配置加载成功: %s", path)
	return &config, nil
}

// GetSessionKeyExpiration 获取 SessionKey 过期时间
func (c *Config) GetSessionKeyExpiration() time.Duration {
	if c.Session.KeyExpirationHours <= 0 {
		return 30 * 24 * time.Hour // 默认30天
	}
	return time.Duration(c.Session.KeyExpirationHours) * time.Hour
}
