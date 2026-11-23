package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig `yaml:"server"`
	Database DBConfig     `yaml:"database"`
}

type ServerConfig struct {
	Port          int    `yaml:"port"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	Secret        string `yaml:"secret"`
	SessionMaxAge int    `yaml:"session_max_age"`
}

type DBConfig struct {
	Path string `yaml:"path"`
}

// OSSConfig 是设置页 / 库中的 OSS 连接参数，不从 yaml 加载。
type OSSConfig struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	Private         bool
	SignedExpire    int
	URLPrefix       string
}

// Load 读取配置文件并补齐默认值
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.SessionMaxAge <= 0 {
		c.Server.SessionMaxAge = 86400
	}
	if c.Server.Secret == "" {
		c.Server.Secret = "assethub-default-secret-change-me"
	}
	if c.Database.Path == "" {
		c.Database.Path = "assethub.db"
	}
	return &c, nil
}
