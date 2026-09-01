// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package config 定义 FasterEdge2Api 服务的运行配置。
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Role 描述本节点在集群中的角色。
type Role string

const (
	// RoleNeutral 中立节点:不注册 Cloud/Edge 角色能力。
	RoleNeutral Role = "neutral"
	// RoleCloud 云端节点:注册 CloudRoleAbility。
	RoleCloud Role = "cloud"
	// RoleEdge 边缘节点:注册 EdgeRoleAbility。
	RoleEdge Role = "edge"
)

// Config 是 FasterEdge2Api 的全部运行配置。
type Config struct {
	// Listen 是 HTTP 监听地址,例如 ":8080"。
	Listen string
	// NodeName 是本节点在集群中的名字(写入 NetMapData)。
	NodeName string
	// Role 是本节点的集群角色(cloud / edge / neutral)。
	Role Role
	// KeyringPath 是 KeyringData 持久化快照路径。
	// 留空表示不持久化(每次重启密钥与令牌重置)。
	KeyringPath string
	// TLSCert/TLSKey 同时提供时启用原生 HTTPS。
	TLSCert string
	TLSKey  string
	// ShutdownTimeout 是优雅退出时等待组件卸载的超时。
	ShutdownTimeout time.Duration
}

// Default 返回一份带默认值的配置。
func Default() Config {
	return Config{
		Listen:          ":8080",
		NodeName:        "edge-1",
		Role:            RoleNeutral,
		KeyringPath:     "",
		ShutdownTimeout: 5 * time.Second,
	}
}

// FromEnv 从环境变量读取配置,未设置项回退到默认值。
// 支持:FE2A_LISTEN / FE2A_NODE_NAME / FE2A_ROLE / FE2A_KEYRING_PATH / FE2A_SHUTDOWN_TIMEOUT。
func FromEnv() (Config, error) {
	cfg := Default()
	if v := os.Getenv("FE2A_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("FE2A_NODE_NAME"); v != "" {
		cfg.NodeName = v
	}
	if v := os.Getenv("FE2A_ROLE"); v != "" {
		role, err := ParseRole(v)
		if err != nil {
			return cfg, err
		}
		cfg.Role = role
	}
	if v := os.Getenv("FE2A_KEYRING_PATH"); v != "" {
		cfg.KeyringPath = v
	}
	cfg.TLSCert = os.Getenv("FE2A_TLS_CERT")
	cfg.TLSKey = os.Getenv("FE2A_TLS_KEY")
	if v := os.Getenv("FE2A_SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid FE2A_SHUTDOWN_TIMEOUT %q: %w", v, err)
		}
		cfg.ShutdownTimeout = d
	}
	return cfg, nil
}

// ParseRole 解析角色字符串,大小写不敏感。
func ParseRole(s string) (Role, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "neutral", "node":
		return RoleNeutral, nil
	case "cloud", "controller":
		return RoleCloud, nil
	case "edge":
		return RoleEdge, nil
	}
	return "", fmt.Errorf("unknown role %q (expected cloud/edge/neutral)", s)
}

// Validate 校验配置合法性。
func (c Config) Validate() error {
	if strings.TrimSpace(c.Listen) == "" {
		return fmt.Errorf("config: listen address is empty")
	}
	if strings.TrimSpace(c.NodeName) == "" {
		return fmt.Errorf("config: node name is empty")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return fmt.Errorf("config: TLS certificate and key must be provided together")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("config: shutdown timeout must be positive")
	}
	return nil
}
