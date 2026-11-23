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

// Load 读取配置文件、补齐默认值，并用非空环境变量覆盖登录相关字段。
// 第二个返回值为被覆盖的字段名（username / password / secret）。
func Load(path string) (*Config, []string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, nil, err
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
	overridden := applyEnvOverrides(&c)
	return &c, overridden, nil
}

func applyEnvOverrides(c *Config) []string {
	var overridden []string
	if v := os.Getenv("ASSETHUB_USERNAME"); v != "" {
		c.Server.Username = v
		overridden = append(overridden, "username")
	}
	if v := os.Getenv("ASSETHUB_PASSWORD"); v != "" {
		c.Server.Password = v
		overridden = append(overridden, "password")
	}
	if v := os.Getenv("ASSETHUB_SECRET"); v != "" {
		c.Server.Secret = v
		overridden = append(overridden, "secret")
	}
	return overridden
}
