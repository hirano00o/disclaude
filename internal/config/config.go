package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config はアプリケーション全体の設定を保持する構造体
type Config struct {
	Discord    DiscordConfig
	Database   DatabaseConfig
	Kubernetes KubernetesConfig
	Claude     ClaudeConfig
}

// DiscordConfig はDiscord Bot関連の設定
type DiscordConfig struct {
	Token   string
	GuildID string
}

// DatabaseConfig はデータベース関連の設定
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// KubernetesConfig はKubernetes関連の設定
type KubernetesConfig struct {
	Namespace    string
	MaxSandboxes int
}

// ClaudeConfig はClaude Code関連の設定
type ClaudeConfig struct {
	APIKey     string
	ConfigPath string
}

// Load は環境変数から設定を読み込む
func Load() (*Config, error) {
	// データベースポートの取得
	portStr := getEnvWithDefault("DB_PORT", "5432")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	// 最大サンドボックス数の取得
	maxSandboxesStr := getEnvWithDefault("MAX_SANDBOXES", "3")
	maxSandboxes, err := strconv.Atoi(maxSandboxesStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_SANDBOXES: %w", err)
	}

	// 必須環境変数の確認
	requiredEnvVars := []string{
		"DISCORD_TOKEN",
		"DB_HOST",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"CLAUDE_API_KEY",
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			return nil, fmt.Errorf("required environment variable %s is not set", envVar)
		}
	}

	config := &Config{
		Discord: DiscordConfig{
			Token:   os.Getenv("DISCORD_TOKEN"),
			GuildID: os.Getenv("DISCORD_GUILD_ID"),
		},
		Database: DatabaseConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     port,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Database: os.Getenv("DB_NAME"),
		},
		Kubernetes: KubernetesConfig{
			Namespace:    getEnvWithDefault("KUBERNETES_NAMESPACE", "discord-claude"),
			MaxSandboxes: maxSandboxes,
		},
		Claude: ClaudeConfig{
			APIKey:     os.Getenv("CLAUDE_API_KEY"),
			ConfigPath: getEnvWithDefault("CLAUDE_CONFIG_PATH", "/home/user/.claude"),
		},
	}

	return config, nil
}

// getEnvWithDefault は環境変数を取得し、存在しない場合はデフォルト値を返す
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetDatabaseURL はデータベース接続URLを生成する
func (c *Config) GetDatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Database,
	)
}