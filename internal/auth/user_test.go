package auth

import (
	"fmt"
	"testing"

	"discord-claude/internal/db"
)

// TestUserServiceInitializeUser はユーザー初期化のテスト
func TestUserServiceInitializeUser(t *testing.T) {
	mockDB := &MockDB{
		users: make(map[string]*db.User),
	}
	service := NewUserService(mockDB)

	// 新規ユーザーのオーナー初期化
	user, err := service.InitializeUser("123456789", "testowner", true)
	if err != nil {
		t.Fatalf("Failed to initialize user as owner: %v", err)
	}

	if user.Role != "owner" {
		t.Errorf("Expected role 'owner', got '%s'", user.Role)
	}

	// 既存ユーザーの場合
	existingUser, err := service.InitializeUser("123456789", "testowner", false)
	if err != nil {
		t.Fatalf("Failed to get existing user: %v", err)
	}

	if existingUser.ID != user.ID {
		t.Errorf("Expected same user ID %d, got %d", user.ID, existingUser.ID)
	}
}

// TestUserServiceAddUser はユーザー追加のテスト
func TestUserServiceAddUser(t *testing.T) {
	mockDB := &MockDB{
		users: make(map[string]*db.User),
	}
	service := NewUserService(mockDB)

	// オーナーを作成
	owner, _ := service.InitializeUser("owner123", "owner", true)

	// 一般ユーザーを追加
	user, err := service.AddUser("owner123", "user123", "testuser")
	if err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}

	if user.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", user.Role)
	}

	// 一般ユーザーが他のユーザーを追加しようとする（失敗すべき）
	_, err = service.AddUser("user123", "user456", "anotheruser")
	if err == nil {
		t.Error("Expected error when non-owner tries to add user")
	}

	// 存在しないユーザーが追加しようとする（失敗すべき）
	_, err = service.AddUser("nonexistent", "user789", "someuser")
	if err == nil {
		t.Error("Expected error when non-existent user tries to add user")
	}
}

// TestUserServicePromoteToOwner はオーナー昇格のテスト
func TestUserServicePromoteToOwner(t *testing.T) {
	mockDB := &MockDB{
		users: make(map[string]*db.User),
	}
	service := NewUserService(mockDB)

	// オーナーと一般ユーザーを作成
	owner, _ := service.InitializeUser("owner123", "owner", true)
	user, _ := service.AddUser("owner123", "user123", "testuser")

	// 一般ユーザーをオーナーに昇格
	err := service.PromoteToOwner("owner123", "user123")
	if err != nil {
		t.Fatalf("Failed to promote user to owner: %v", err)
	}

	// 更新されたユーザー情報を確認
	updatedUser, _ := service.GetUser("user123")
	if updatedUser.Role != "owner" {
		t.Errorf("Expected role 'owner', got '%s'", updatedUser.Role)
	}

	// 一般ユーザーが昇格を試行（失敗すべき）
	err = service.PromoteToOwner("user123", "owner123")
	if err == nil {
		t.Error("Expected error when non-owner tries to promote user")
	}
}

// TestUserServiceDemoteFromOwner はオーナー降格のテスト
func TestUserServiceDemoteFromOwner(t *testing.T) {
	mockDB := &MockDB{
		users: make(map[string]*db.User),
	}
	service := NewUserService(mockDB)

	// 2人のオーナーを作成
	owner1, _ := service.InitializeUser("owner1", "owner1", true)
	owner2, _ := service.InitializeUser("owner2", "owner2", true)

	// オーナー1がオーナー2を降格
	err := service.DemoteFromOwner("owner1", "owner2")
	if err != nil {
		t.Fatalf("Failed to demote owner: %v", err)
	}

	// 更新されたユーザー情報を確認
	demotedUser, _ := service.GetUser("owner2")
	if demotedUser.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", demotedUser.Role)
	}

	// 自分自身を降格しようとする（失敗すべき）
	err = service.DemoteFromOwner("owner1", "owner1")
	if err == nil {
		t.Error("Expected error when owner tries to demote themselves")
	}
}

// TestUserServiceRemoveUser はユーザー削除のテスト
func TestUserServiceRemoveUser(t *testing.T) {
	mockDB := &MockDB{
		users: make(map[string]*db.User),
	}
	service := NewUserService(mockDB)

	// オーナーと一般ユーザーを作成
	owner, _ := service.InitializeUser("owner123", "owner", true)
	user, _ := service.AddUser("owner123", "user123", "testuser")

	// ユーザーを削除
	err := service.RemoveUser("owner123", "user123")
	if err != nil {
		t.Fatalf("Failed to remove user: %v", err)
	}

	// 削除されたユーザーが取得できないことを確認
	deletedUser, _ := service.GetUser("user123")
	if deletedUser != nil {
		t.Error("Expected user to be deleted")
	}

	// 自分自身を削除しようとする（失敗すべき）
	err = service.RemoveUser("owner123", "owner123")
	if err == nil {
		t.Error("Expected error when owner tries to remove themselves")
	}
}

// MockDB はテスト用のモックデータベース
type MockDB struct {
	users   map[string]*db.User
	nextID  int
}

func (m *MockDB) GetUserByDiscordID(discordID string) (*db.User, error) {
	user, exists := m.users[discordID]
	if !exists {
		return nil, nil
	}
	return user, nil
}

func (m *MockDB) CreateUser(discordID, username, role string) (*db.User, error) {
	m.nextID++
	user := &db.User{
		ID:        m.nextID,
		DiscordID: discordID,
		Username:  username,
		Role:      role,
	}
	m.users[discordID] = user
	return user, nil
}

func (m *MockDB) UpdateUserRole(discordID, role string) error {
	user, exists := m.users[discordID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	user.Role = role
	return nil
}

func (m *MockDB) DeleteUser(discordID string) error {
	_, exists := m.users[discordID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	delete(m.users, discordID)
	return nil
}