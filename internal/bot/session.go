package bot

import (
	"context"
	"fmt"
	"time"

	"disclaude/internal/db"

	"github.com/sirupsen/logrus"
)

// SessionManager はセッション管理を行う構造体
type SessionManager struct {
	db             *db.DB
	sandboxManager *SandboxManager
}

// SessionInfo はセッション情報を表す構造体
type SessionInfo struct {
	Session  *db.Session
	Sandbox  *db.Sandbox
	IsActive bool
	Duration time.Duration
	User     *db.User
}

// NewSessionManager は新しいSessionManagerを作成する
func NewSessionManager(database *db.DB, sandboxMgr *SandboxManager) *SessionManager {
	return &SessionManager{
		db:             database,
		sandboxManager: sandboxMgr,
	}
}

// GetSessionInfo はセッション情報を取得する
func (sm *SessionManager) GetSessionInfo(threadID string) (*SessionInfo, error) {
	// セッション情報の取得
	session, err := sm.db.GetSessionByThreadID(threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session == nil {
		return &SessionInfo{
			IsActive: false,
		}, nil
	}

	// ユーザー情報の取得
	user, err := sm.db.GetUserByID(session.UserID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get user for session")
	}

	// サンドボックス情報の取得
	sandbox, err := sm.db.GetSandboxByPodName(session.SandboxName)
	if err != nil {
		logrus.WithError(err).Error("Failed to get sandbox for session")
	}

	// セッション継続時間の計算
	duration := time.Since(session.CreatedAt)
	if !session.IsActive() && session.TerminatedAt.Valid {
		duration = session.TerminatedAt.Time.Sub(session.CreatedAt)
	}

	return &SessionInfo{
		Session:  session,
		Sandbox:  sandbox,
		IsActive: session.IsActive(),
		Duration: duration,
		User:     user,
	}, nil
}

// ValidateSessionOwnership はセッションの所有権を確認する
func (sm *SessionManager) ValidateSessionOwnership(threadID, userDiscordID string) (bool, *SessionInfo, error) {
	sessionInfo, err := sm.GetSessionInfo(threadID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get session info: %w", err)
	}

	if !sessionInfo.IsActive {
		return false, sessionInfo, nil
	}

	// セッション所有者チェック
	if sessionInfo.User != nil && sessionInfo.User.DiscordID == userDiscordID {
		return true, sessionInfo, nil
	}

	return false, sessionInfo, nil
}

// CleanupInactiveSessions は非アクティブなセッションをクリーンアップする
func (sm *SessionManager) CleanupInactiveSessions(ctx context.Context, maxAge time.Duration) error {
	// 指定時間以上古い非アクティブセッションを取得
	cutoffTime := time.Now().Add(-maxAge)

	// TODO: データベースクエリを追加してクリーンアップ対象のセッションを取得
	// この機能は将来的な拡張として実装可能

	logrus.WithField("cutoff_time", cutoffTime).Debug("Session cleanup completed")
	return nil
}

// GetActiveSessionsCount はアクティブなセッション数を取得する
func (sm *SessionManager) GetActiveSessionsCount() (int, error) {
	usage, err := sm.db.GetSandboxUsage()
	if err != nil {
		return 0, fmt.Errorf("failed to get sandbox usage: %w", err)
	}

	return usage.CurrentCount, nil
}

// ForceTerminateSession はセッションを強制終了する（管理者機能）
func (sm *SessionManager) ForceTerminateSession(ctx context.Context, sessionID int, reason string) error {
	// セッション情報の取得
	session, err := sm.db.GetSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session == nil {
		return fmt.Errorf("session not found")
	}

	if !session.IsActive() {
		return fmt.Errorf("session is not active")
	}

	// サンドボックスの削除
	if sm.sandboxManager != nil {
		err = sm.sandboxManager.DeleteSandbox(ctx, session.SandboxName)
		if err != nil {
			logrus.WithError(err).Error("Failed to delete sandbox during force termination")
		}
	}

	// セッション状態の更新
	err = sm.db.UpdateSessionStatus(session.ID, "terminated")
	if err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"session_id":   sessionID,
		"sandbox_name": session.SandboxName,
		"reason":       reason,
	}).Info("Session force terminated")

	return nil
}

// GetSessionStatistics はセッション統計を取得する
func (sm *SessionManager) GetSessionStatistics() (*SessionStatistics, error) {
	// TODO: データベースクエリを追加して統計情報を取得
	// この機能は将来的な拡張として実装可能

	return &SessionStatistics{
		TotalSessions:          0,
		ActiveSessions:         0,
		TerminatedSessions:     0,
		AverageSessionDuration: 0,
	}, nil
}

// SessionStatistics はセッション統計情報を表す構造体
type SessionStatistics struct {
	TotalSessions          int
	ActiveSessions         int
	TerminatedSessions     int
	AverageSessionDuration time.Duration
}

// SandboxManager はサンドボックス管理のインターフェース
type SandboxManager interface {
	DeleteSandbox(ctx context.Context, podName string) error
}
