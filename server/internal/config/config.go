package config

// 解析 config.yml(端口、DB 路径、JWT 密钥)。

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// 各分组用具名类型而不是匿名结构体:这样它们能被单独传递、加方法(如校验),
// 也让 Config 本体保持简短可读。YAML 结构不受影响,仍是 server / database / jwt 三段。

// Server HTTP 服务配置(监听地址等)。
type Server struct {
	Addr string `yaml:"addr"`
}

// Database 数据库配置。
type Database struct {
	Path string `yaml:"path"`
}

// JWT 签名配置。
type JWT struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type Config struct {
	Server   Server   `yaml:"server"`
	Database Database `yaml:"database"`
	JWT      JWT      `yaml:"jwt"`
}

// Load 读取并解析 config.yml:读文件 → 解析 → 填默认值 → 校验,任一步失败都带上下文返回。
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.Default()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Default 填充缺省值,为默认数据
func (c *Config) Default() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Database.Path == "" {
		c.Database.Path = "./data/database.db"
	}
	if c.JWT.ExpireHours == 0 {
		c.JWT.ExpireHours = 72
	}
}

// Validate 校验必填项,否则不启动
func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		return errors.New("config: jwt.secret is required (an empty secret makes tokens forgeable)")
	}
	return nil
}
