package auth

import (
	"fmt"

	"discord-claude/internal/db"
)

// UserService はユーザー認証・管理を行うサービス
type UserService struct {
	db *db.DB
}

// NewUserService は新しいUserServiceを作成する
func NewUserService(database *db.DB) *UserService {
	return &UserService{
		db: database,
	}
}

// InitializeUser は初回ユーザーの初期化を行う
// 初回ユーザーがオーナー確認を行い、オーナーまたは一般ユーザーとして登録される
func (s *UserService) InitializeUser(discordID, username string, isOwner bool) (*db.User, error) {
	// 既存ユーザーチェック
	existingUser, err := s.db.GetUserByDiscordID(discordID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	
	if existingUser != nil {
		return existingUser, nil
	}

	// ロールの決定
	role := "user"
	if isOwner {
		role = "owner"
	}

	// ユーザー作成
	user, err := s.db.CreateUser(discordID, username, role)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUser はDiscord IDでユーザーを取得する
func (s *UserService) GetUser(discordID string) (*db.User, error) {
	user, err := s.db.GetUserByDiscordID(discordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return user, nil
}

// IsUserExists はユーザーが存在するかチェックする
func (s *UserService) IsUserExists(discordID string) (bool, error) {
	user, err := s.GetUser(discordID)
	if err != nil {
		return false, err
	}
	
	return user != nil, nil
}

// AddUser は新しいユーザーを追加する（オーナーのみ実行可能）
func (s *UserService) AddUser(requesterDiscordID, targetDiscordID, targetUsername string) (*db.User, error) {
	// 要求者の権限チェック
	requester, err := s.GetUser(requesterDiscordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get requester: %w", err)
	}
	
	if requester == nil {
		return nil, fmt.Errorf("requester not found")
	}
	
	if !requester.IsOwner() {
		return nil, fmt.Errorf("insufficient permissions: only owners can add users")
	}

	// 対象ユーザーの重複チェック
	existingUser, err := s.GetUser(targetDiscordID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing target user: %w", err)
	}
	
	if existingUser != nil {
		return nil, fmt.Errorf("user already exists")
	}

	// ユーザー作成
	user, err := s.db.CreateUser(targetDiscordID, targetUsername, "user")
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// PromoteToOwner はユーザーをオーナーに昇格させる（オーナーのみ実行可能）
func (s *UserService) PromoteToOwner(requesterDiscordID, targetDiscordID string) error {
	// 要求者の権限チェック
	requester, err := s.GetUser(requesterDiscordID)
	if err != nil {
		return fmt.Errorf("failed to get requester: %w", err)
	}
	
	if requester == nil {
		return fmt.Errorf("requester not found")
	}
	
	if !requester.IsOwner() {
		return fmt.Errorf("insufficient permissions: only owners can promote users")
	}

	// 対象ユーザーの存在チェック
	targetUser, err := s.GetUser(targetDiscordID)
	if err != nil {
		return fmt.Errorf("failed to get target user: %w", err)
	}
	
	if targetUser == nil {
		return fmt.Errorf("target user not found")
	}
	
	if targetUser.IsOwner() {
		return fmt.Errorf("user is already an owner")
	}

	// ロール更新
	if err := s.db.UpdateUserRole(targetDiscordID, "owner"); err != nil {
		return fmt.Errorf("failed to promote user to owner: %w", err)
	}

	return nil
}

// DemoteFromOwner はオーナーを一般ユーザーに降格させる（オーナーのみ実行可能、自分自身は不可）
func (s *UserService) DemoteFromOwner(requesterDiscordID, targetDiscordID string) error {
	// 要求者の権限チェック
	requester, err := s.GetUser(requesterDiscordID)
	if err != nil {
		return fmt.Errorf("failed to get requester: %w", err)
	}
	
	if requester == nil {
		return fmt.Errorf("requester not found")
	}
	
	if !requester.IsOwner() {
		return fmt.Errorf("insufficient permissions: only owners can demote users")
	}

	// 自分自身の降格防止
	if requesterDiscordID == targetDiscordID {
		return fmt.Errorf("cannot demote yourself")
	}

	// 対象ユーザーの存在チェック
	targetUser, err := s.GetUser(targetDiscordID)
	if err != nil {
		return fmt.Errorf("failed to get target user: %w", err)
	}
	
	if targetUser == nil {
		return fmt.Errorf("target user not found")
	}
	
	if !targetUser.IsOwner() {
		return fmt.Errorf("user is not an owner")
	}

	// ロール更新
	if err := s.db.UpdateUserRole(targetDiscordID, "user"); err != nil {
		return fmt.Errorf("failed to demote user from owner: %w", err)
	}

	return nil
}

// RemoveUser はユーザーを削除する（オーナーのみ実行可能、自分自身は不可）
func (s *UserService) RemoveUser(requesterDiscordID, targetDiscordID string) error {
	// 要求者の権限チェック
	requester, err := s.GetUser(requesterDiscordID)
	if err != nil {
		return fmt.Errorf("failed to get requester: %w", err)
	}
	
	if requester == nil {
		return fmt.Errorf("requester not found")
	}
	
	if !requester.IsOwner() {
		return fmt.Errorf("insufficient permissions: only owners can remove users")
	}

	// 自分自身の削除防止
	if requesterDiscordID == targetDiscordID {
		return fmt.Errorf("cannot remove yourself")
	}

	// 対象ユーザーの存在チェック
	targetUser, err := s.GetUser(targetDiscordID)
	if err != nil {
		return fmt.Errorf("failed to get target user: %w", err)
	}
	
	if targetUser == nil {
		return fmt.Errorf("target user not found")
	}

	// ユーザー削除
	if err := s.db.DeleteUser(targetDiscordID); err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}

	return nil
}