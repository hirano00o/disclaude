package db

import (
	"testing"

	_ "github.com/lib/pq"
)

// TestUserCRUD はユーザー操作のテスト
func TestUserCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// ユーザー作成テスト
	user, err := db.CreateUser("123456789", "testuser", "user")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.DiscordID != "123456789" {
		t.Errorf("Expected DiscordID '123456789', got '%s'", user.DiscordID)
	}

	if user.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", user.Role)
	}

	// ユーザー取得テスト
	retrievedUser, err := db.GetUserByDiscordID("123456789")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if retrievedUser == nil {
		t.Fatal("Expected user to be found, got nil")
	}

	if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %d, got %d", user.ID, retrievedUser.ID)
	}

	// ユーザーロール更新テスト
	err = db.UpdateUserRole("123456789", "owner")
	if err != nil {
		t.Fatalf("Failed to update user role: %v", err)
	}

	updatedUser, err := db.GetUserByDiscordID("123456789")
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if updatedUser.Role != "owner" {
		t.Errorf("Expected role 'owner', got '%s'", updatedUser.Role)
	}

	// ユーザー削除テスト
	err = db.DeleteUser("123456789")
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	deletedUser, err := db.GetUserByDiscordID("123456789")
	if err != nil {
		t.Fatalf("Failed to check deleted user: %v", err)
	}

	if deletedUser != nil {
		t.Error("Expected user to be deleted, but still exists")
	}
}

// TestSessionCRUD はセッション操作のテスト
func TestSessionCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// テスト用ユーザー作成
	user, err := db.CreateUser("123456789", "testuser", "user")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// セッション作成テスト
	session, err := db.CreateSession(user.ID, "thread123", "sandbox-test")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.ThreadID != "thread123" {
		t.Errorf("Expected ThreadID 'thread123', got '%s'", session.ThreadID)
	}

	if session.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", session.Status)
	}

	// セッション取得テスト
	retrievedSession, err := db.GetSessionByThreadID("thread123")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrievedSession == nil {
		t.Fatal("Expected session to be found, got nil")
	}

	if retrievedSession.ID != session.ID {
		t.Errorf("Expected session ID %d, got %d", session.ID, retrievedSession.ID)
	}

	// セッションステータス更新テスト
	err = db.UpdateSessionStatus(session.ID, "terminated")
	if err != nil {
		t.Fatalf("Failed to update session status: %v", err)
	}

	updatedSession, err := db.GetSessionByThreadID("thread123")
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}

	if updatedSession.Status != "terminated" {
		t.Errorf("Expected status 'terminated', got '%s'", updatedSession.Status)
	}

	if !updatedSession.TerminatedAt.Valid {
		t.Error("Expected TerminatedAt to be set")
	}
}

// TestSandboxCRUD はサンドボックス操作のテスト
func TestSandboxCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// テスト用ユーザーとセッション作成
	user, err := db.CreateUser("123456789", "testuser", "user")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	session, err := db.CreateSession(user.ID, "thread123", "sandbox-test")
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// サンドボックス作成テスト
	sandbox, err := db.CreateSandbox(session.ID, "test-pod", "disclaude")
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	if sandbox.PodName != "test-pod" {
		t.Errorf("Expected PodName 'test-pod', got '%s'", sandbox.PodName)
	}

	if sandbox.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", sandbox.Status)
	}

	// サンドボックス取得テスト
	retrievedSandbox, err := db.GetSandboxByPodName("test-pod")
	if err != nil {
		t.Fatalf("Failed to get sandbox: %v", err)
	}

	if retrievedSandbox == nil {
		t.Fatal("Expected sandbox to be found, got nil")
	}

	// サンドボックスステータス更新テスト
	err = db.UpdateSandboxStatus(sandbox.ID, "running")
	if err != nil {
		t.Fatalf("Failed to update sandbox status: %v", err)
	}

	updatedSandbox, err := db.GetSandboxByPodName("test-pod")
	if err != nil {
		t.Fatalf("Failed to get updated sandbox: %v", err)
	}

	if updatedSandbox.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", updatedSandbox.Status)
	}
}

// TestSandboxUsage はサンドボックス使用量管理のテスト
func TestSandboxUsage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// 初期使用量取得
	usage, err := db.GetSandboxUsage()
	if err != nil {
		t.Fatalf("Failed to get sandbox usage: %v", err)
	}

	initialCount := usage.CurrentCount

	// 使用量増加テスト
	err = db.IncrementSandboxUsage()
	if err != nil {
		t.Fatalf("Failed to increment sandbox usage: %v", err)
	}

	usage, err = db.GetSandboxUsage()
	if err != nil {
		t.Fatalf("Failed to get sandbox usage after increment: %v", err)
	}

	if usage.CurrentCount != initialCount+1 {
		t.Errorf("Expected current count %d, got %d", initialCount+1, usage.CurrentCount)
	}

	// 使用量減少テスト
	err = db.DecrementSandboxUsage()
	if err != nil {
		t.Fatalf("Failed to decrement sandbox usage: %v", err)
	}

	usage, err = db.GetSandboxUsage()
	if err != nil {
		t.Fatalf("Failed to get sandbox usage after decrement: %v", err)
	}

	if usage.CurrentCount != initialCount {
		t.Errorf("Expected current count %d, got %d", initialCount, usage.CurrentCount)
	}

	// 容量制限チェックテスト
	if !usage.CanCreateSandbox() && usage.CurrentCount < usage.MaxCount {
		t.Error("Expected to be able to create sandbox")
	}

	if usage.CanCreateSandbox() && usage.CurrentCount >= usage.MaxCount {
		t.Error("Expected not to be able to create sandbox")
	}
}

// setupTestDB はテスト用データベースをセットアップする
func setupTestDB(t *testing.T) *DB {
	// テスト用データベース接続
	// 実際のテストでは環境変数やテスト用DBを使用
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "test_user",
		Password: "test_password",
		Database: "test_disclaude",
	}

	db, err := NewConnection(config)
	if err != nil {
		t.Skipf("Failed to connect to test database: %v", err)
	}

	// テーブルが存在しない場合は作成
	err = Migrate(db)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	// テストデータのクリーンアップ
	cleanupTestData(t, db)

	return db
}

// cleanupTestData はテストデータをクリーンアップする
func cleanupTestData(t *testing.T, db *DB) {
	// 外部キー制約を考慮した順序で削除
	tables := []string{"sandboxes", "sessions", "users"}

	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table + " WHERE created_at < NOW()")
		if err != nil {
			t.Logf("Warning: Failed to cleanup table %s: %v", table, err)
		}
	}

	// サンドボックス使用量をリセット
	_, err := db.Exec("UPDATE sandbox_usage SET current_count = 0")
	if err != nil {
		t.Logf("Warning: Failed to reset sandbox usage: %v", err)
	}
}
