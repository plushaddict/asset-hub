package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig `yaml:"server"`
	Database DBConfig     `yaml:"database"`
	OSS      OSSConfig    `yaml:"oss"`
}

type ServerConfig struct {
	Port          int    `yaml:"port"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	Secret        string `yaml:"secret"`
	SessionMaxAge int    `yaml:"session_max_age"`
	BusinessToken string `yaml:"business_token"`
}

type DBConfig struct {
	Path string `yaml:"path"`
}

type OSSConfig struct {
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Private         bool   `yaml:"private"`
	SignedExpire    int    `yaml:"signed_expire"`
	URLPrefix       string `yaml:"url_prefix"`
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
	if c.OSS.SignedExpire <= 0 {
		c.OSS.SignedExpire = 3600
	}
	return &c, nil
}