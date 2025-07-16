package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// DB はデータベース接続を管理する構造体
type DB struct {
	*sql.DB
}

// NewConnection は新しいデータベース接続を作成する
func NewConnection(config DatabaseConfig) (*DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// 接続テスト
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// 接続プールの設定
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(time.Hour)

	return &DB{db}, nil
}

// DatabaseConfig はデータベース設定を表す構造体
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// Migrate はデータベースマイグレーションを実行する
func Migrate(db *DB) error {
	schemaFile := filepath.Join("sql", "schema.sql")
	schema, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	logrus.Info("Database migration completed successfully")
	return nil
}

// CreateUser は新しいユーザーを作成する
func (db *DB) CreateUser(discordID, username, role string) (*User, error) {
	query := `
		INSERT INTO users (discord_id, username, role)
		VALUES ($1, $2, $3)
		RETURNING id, discord_id, username, role, created_at, updated_at
	`
	
	user := &User{}
	err := db.QueryRow(query, discordID, username, role).Scan(
		&user.ID,
		&user.DiscordID,
		&user.Username,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByDiscordID はDiscord IDでユーザーを取得する
func (db *DB) GetUserByDiscordID(discordID string) (*User, error) {
	query := `
		SELECT id, discord_id, username, role, created_at, updated_at
		FROM users
		WHERE discord_id = $1
	`
	
	user := &User{}
	err := db.QueryRow(query, discordID).Scan(
		&user.ID,
		&user.DiscordID,
		&user.Username,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// UpdateUserRole はユーザーの役割を更新する
func (db *DB) UpdateUserRole(discordID, role string) error {
	query := `
		UPDATE users
		SET role = $1
		WHERE discord_id = $2
	`
	
	result, err := db.Exec(query, role, discordID)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser はユーザーを削除する
func (db *DB) DeleteUser(discordID string) error {
	query := `DELETE FROM users WHERE discord_id = $1`
	
	result, err := db.Exec(query, discordID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// CreateSession は新しいセッションを作成する
func (db *DB) CreateSession(userID int, threadID, sandboxName string) (*Session, error) {
	query := `
		INSERT INTO sessions (user_id, thread_id, sandbox_name, status)
		VALUES ($1, $2, $3, 'active')
		RETURNING id, user_id, thread_id, sandbox_name, status, created_at, updated_at
	`
	
	session := &Session{}
	err := db.QueryRow(query, userID, threadID, sandboxName).Scan(
		&session.ID,
		&session.UserID,
		&session.ThreadID,
		&session.SandboxName,
		&session.Status,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GetSessionByThreadID はスレッドIDでセッションを取得する
func (db *DB) GetSessionByThreadID(threadID string) (*Session, error) {
	query := `
		SELECT id, user_id, thread_id, sandbox_name, status, created_at, updated_at, terminated_at
		FROM sessions
		WHERE thread_id = $1
	`
	
	session := &Session{}
	err := db.QueryRow(query, threadID).Scan(
		&session.ID,
		&session.UserID,
		&session.ThreadID,
		&session.SandboxName,
		&session.Status,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.TerminatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// UpdateSessionStatus はセッションのステータスを更新する
func (db *DB) UpdateSessionStatus(sessionID int, status string) error {
	query := `
		UPDATE sessions
		SET status = $1, terminated_at = CASE WHEN $1 = 'terminated' THEN CURRENT_TIMESTAMP ELSE NULL END
		WHERE id = $2
	`
	
	_, err := db.Exec(query, status, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	return nil
}

// CreateSandbox は新しいサンドボックスを作成する
func (db *DB) CreateSandbox(sessionID int, podName, namespace string) (*Sandbox, error) {
	query := `
		INSERT INTO sandboxes (session_id, pod_name, namespace, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, session_id, pod_name, namespace, cpu_limit, memory_limit, status, created_at, updated_at
	`
	
	sandbox := &Sandbox{}
	err := db.QueryRow(query, sessionID, podName, namespace).Scan(
		&sandbox.ID,
		&sandbox.SessionID,
		&sandbox.PodName,
		&sandbox.Namespace,
		&sandbox.CPULimit,
		&sandbox.MemoryLimit,
		&sandbox.Status,
		&sandbox.CreatedAt,
		&sandbox.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	return sandbox, nil
}

// GetSandboxByPodName はPod名でサンドボックスを取得する
func (db *DB) GetSandboxByPodName(podName string) (*Sandbox, error) {
	query := `
		SELECT id, session_id, pod_name, namespace, cpu_limit, memory_limit, status, created_at, updated_at
		FROM sandboxes
		WHERE pod_name = $1
	`
	
	sandbox := &Sandbox{}
	err := db.QueryRow(query, podName).Scan(
		&sandbox.ID,
		&sandbox.SessionID,
		&sandbox.PodName,
		&sandbox.Namespace,
		&sandbox.CPULimit,
		&sandbox.MemoryLimit,
		&sandbox.Status,
		&sandbox.CreatedAt,
		&sandbox.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get sandbox: %w", err)
	}

	return sandbox, nil
}

// UpdateSandboxStatus はサンドボックスのステータスを更新する
func (db *DB) UpdateSandboxStatus(sandboxID int, status string) error {
	query := `
		UPDATE sandboxes
		SET status = $1
		WHERE id = $2
	`
	
	_, err := db.Exec(query, status, sandboxID)
	if err != nil {
		return fmt.Errorf("failed to update sandbox status: %w", err)
	}

	return nil
}

// GetSandboxUsage はサンドボックスの使用状況を取得する
func (db *DB) GetSandboxUsage() (*SandboxUsage, error) {
	query := `
		SELECT id, current_count, max_count, updated_at
		FROM sandbox_usage
		LIMIT 1
	`
	
	usage := &SandboxUsage{}
	err := db.QueryRow(query).Scan(
		&usage.ID,
		&usage.CurrentCount,
		&usage.MaxCount,
		&usage.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get sandbox usage: %w", err)
	}

	return usage, nil
}

// IncrementSandboxUsage はサンドボックスの使用数を増加させる
func (db *DB) IncrementSandboxUsage() error {
	query := `
		UPDATE sandbox_usage
		SET current_count = current_count + 1
		WHERE id = 1
	`
	
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to increment sandbox usage: %w", err)
	}

	return nil
}

// DecrementSandboxUsage はサンドボックスの使用数を減少させる
func (db *DB) DecrementSandboxUsage() error {
	query := `
		UPDATE sandbox_usage
		SET current_count = GREATEST(current_count - 1, 0)
		WHERE id = 1
	`
	
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to decrement sandbox usage: %w", err)
	}

	return nil
}