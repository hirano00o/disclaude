package db

import (
	"database/sql"
	"time"
)

// User はユーザー情報を表すモデル
type User struct {
	ID        int       `db:"id"`
	DiscordID string    `db:"discord_id"`
	Username  string    `db:"username"`
	Role      string    `db:"role"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Session はセッション情報を表すモデル
type Session struct {
	ID           int            `db:"id"`
	UserID       int            `db:"user_id"`
	ThreadID     string         `db:"thread_id"`
	SandboxName  string         `db:"sandbox_name"`
	Status       string         `db:"status"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
	TerminatedAt sql.NullTime   `db:"terminated_at"`
}

// Sandbox はサンドボックス情報を表すモデル
type Sandbox struct {
	ID          int       `db:"id"`
	SessionID   int       `db:"session_id"`
	PodName     string    `db:"pod_name"`
	Namespace   string    `db:"namespace"`
	CPULimit    string    `db:"cpu_limit"`
	MemoryLimit string    `db:"memory_limit"`
	Status      string    `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// SandboxUsage はサンドボックス使用状況を表すモデル
type SandboxUsage struct {
	ID           int       `db:"id"`
	CurrentCount int       `db:"current_count"`
	MaxCount     int       `db:"max_count"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// IsOwner はユーザーがオーナーかどうかを判定する
func (u *User) IsOwner() bool {
	return u.Role == "owner"
}

// IsUser はユーザーが一般ユーザーかどうかを判定する
func (u *User) IsUser() bool {
	return u.Role == "user"
}

// IsActive はセッションがアクティブかどうかを判定する
func (s *Session) IsActive() bool {
	return s.Status == "active"
}

// IsTerminated はセッションが終了しているかどうかを判定する
func (s *Session) IsTerminated() bool {
	return s.Status == "terminated"
}

// IsRunning はサンドボックスが実行中かどうかを判定する
func (sb *Sandbox) IsRunning() bool {
	return sb.Status == "running"
}

// IsPending はサンドボックスが保留中かどうかを判定する
func (sb *Sandbox) IsPending() bool {
	return sb.Status == "pending"
}

// IsTerminated はサンドボックスが終了しているかどうかを判定する
func (sb *Sandbox) IsTerminated() bool {
	return sb.Status == "terminated"
}

// CanCreateSandbox はサンドボックスを作成できるかどうかを判定する
func (su *SandboxUsage) CanCreateSandbox() bool {
	return su.CurrentCount < su.MaxCount
}

// RemainingCapacity は残りのサンドボックス作成可能数を返す
func (su *SandboxUsage) RemainingCapacity() int {
	return su.MaxCount - su.CurrentCount
}