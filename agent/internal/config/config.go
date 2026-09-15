package config

// 解析 config.yml(api/mode/token/scheduler/stack/render),并做默认值 + 校验。
// 详见 需求文档.md 4.4 配置示例。

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeNode   Mode = "node"
	ModeClient Mode = "client"
)

// Render 仅 client 模式使用:渲染生成配置文件的目标。
type Render struct {
	Type string `yaml:"type"` // soga
	Path string `yaml:"path"`
}

type Config struct {
	API       string `yaml:"api"`       // Server 基址,如 https://panel.example.com
	Mode      Mode   `yaml:"mode"`      // node | client
	Token     string `yaml:"token"`     // node:节点专属 token;client:全局只读 token
	Scheduler string `yaml:"scheduler"` // cron(6 段含秒)
	Stack     string `yaml:"stack"`     // client 专用:default | 4 | 6
	Render    Render `yaml:"render"`    // client 专用
}

// Load 读取并解析配置文件。
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

// Default 填充缺省值(并顺手 trim 掉各字段首尾空白)。
func (c *Config) Default() {
	c.API = strings.TrimRight(strings.TrimSpace(c.API), "/")
	c.Mode = Mode(strings.TrimSpace(string(c.Mode)))
	c.Token = strings.TrimSpace(c.Token)
	c.Scheduler = strings.TrimSpace(c.Scheduler)
	c.Stack = strings.TrimSpace(c.Stack)
	if c.Mode == ModeClient && c.Stack == "" {
		c.Stack = "default"
	}
}

// Validate 校验必填项与取值范围,不合法则不启动。
func (c *Config) Validate() error {
	if c.API == "" {
		return errors.New("config: api is required")
	}
	switch c.Mode {
	case ModeNode, ModeClient:
	default:
		return fmt.Errorf("config: mode must be %q or %q, got %q", ModeNode, ModeClient, c.Mode)
	}
	if c.Token == "" {
		return errors.New("config: token is required")
	}
	if c.Scheduler == "" {
		return errors.New("config: scheduler (cron) is required")
	}
	if c.Mode == ModeClient {
		switch c.Stack {
		case "default", "4", "6":
		default:
			return fmt.Errorf("config: stack must be default|4|6, got %q", c.Stack)
		}
		if c.Render.Type == "" || c.Render.Path == "" {
			return errors.New("config: client mode requires render.type and render.path")
		}
		switch c.Render.Type {
		case "soga":
		default:
			return fmt.Errorf("config: render.type must be soga (xrayr 已废弃), got %q", c.Render.Type)
		}
	}
	return nil
}
