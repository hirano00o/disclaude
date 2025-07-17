package auth

import (
	"fmt"

	"disclaude/internal/db"
)

// PermissionService は権限管理を行うサービス
type PermissionService struct {
	db *db.DB
}

// NewPermissionService は新しいPermissionServiceを作成する
func NewPermissionService(database *db.DB) *PermissionService {
	return &PermissionService{
		db: database,
	}
}

// Permission は権限レベルを表す列挙型
type Permission int

const (
	// PermissionNone は権限なし
	PermissionNone Permission = iota
	// PermissionUser は一般ユーザー権限
	PermissionUser
	// PermissionOwner はオーナー権限
	PermissionOwner
)

// GetUserPermission はユーザーの権限レベルを取得する
func (s *PermissionService) GetUserPermission(discordID string) (Permission, error) {
	user, err := s.db.GetUserByDiscordID(discordID)
	if err != nil {
		return PermissionNone, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return PermissionNone, nil
	}

	switch user.Role {
	case "owner":
		return PermissionOwner, nil
	case "user":
		return PermissionUser, nil
	default:
		return PermissionNone, nil
	}
}

// CanCreateSandbox はサンドボックスを作成できるかチェックする
func (s *PermissionService) CanCreateSandbox(discordID string) (bool, error) {
	permission, err := s.GetUserPermission(discordID)
	if err != nil {
		return false, err
	}

	return permission >= PermissionUser, nil
}

// CanManageUsers はユーザー管理ができるかチェックする
func (s *PermissionService) CanManageUsers(discordID string) (bool, error) {
	permission, err := s.GetUserPermission(discordID)
	if err != nil {
		return false, err
	}

	return permission >= PermissionOwner, nil
}

// CanDeleteSandbox はサンドボックスを削除できるかチェックする
func (s *PermissionService) CanDeleteSandbox(discordID string, sessionUserID int) (bool, error) {
	permission, err := s.GetUserPermission(discordID)
	if err != nil {
		return false, err
	}

	// オーナーは全てのサンドボックスを削除可能
	if permission >= PermissionOwner {
		return true, nil
	}

	// 一般ユーザーは自分のサンドボックスのみ削除可能
	if permission >= PermissionUser {
		user, err := s.db.GetUserByDiscordID(discordID)
		if err != nil {
			return false, fmt.Errorf("failed to get user: %w", err)
		}

		if user != nil && user.ID == sessionUserID {
			return true, nil
		}
	}

	return false, nil
}

// RequirePermission は必要な権限レベルをチェックし、不足している場合はエラーを返す
func (s *PermissionService) RequirePermission(discordID string, requiredPermission Permission) error {
	permission, err := s.GetUserPermission(discordID)
	if err != nil {
		return fmt.Errorf("failed to get user permission: %w", err)
	}

	if permission < requiredPermission {
		switch requiredPermission {
		case PermissionUser:
			return fmt.Errorf("ユーザー権限が必要です")
		case PermissionOwner:
			return fmt.Errorf("オーナー権限が必要です")
		default:
			return fmt.Errorf("十分な権限がありません")
		}
	}

	return nil
}

// IsOwner はユーザーがオーナーかチェックする
func (s *PermissionService) IsOwner(discordID string) (bool, error) {
	permission, err := s.GetUserPermission(discordID)
	if err != nil {
		return false, err
	}

	return permission == PermissionOwner, nil
}

// IsUser はユーザーが一般ユーザー以上の権限を持っているかチェックする
func (s *PermissionService) IsUser(discordID string) (bool, error) {
	permission, err := s.GetUserPermission(discordID)
	if err != nil {
		return false, err
	}

	return permission >= PermissionUser, nil
}

// ValidateUserAction はユーザーの行動が権限的に許可されているかチェックする
func (s *PermissionService) ValidateUserAction(discordID string, action string) error {
	permission, err := s.GetUserPermission(discordID)
	if err != nil {
		return fmt.Errorf("failed to get user permission: %w", err)
	}

	switch action {
	case "create_sandbox", "close_sandbox":
		if permission < PermissionUser {
			return fmt.Errorf("サンドボックス操作にはユーザー権限が必要です")
		}
	case "add_user", "add_owner", "delete_user", "delete_owner":
		if permission < PermissionOwner {
			return fmt.Errorf("ユーザー管理操作にはオーナー権限が必要です")
		}
	default:
		return fmt.Errorf("不明な操作: %s", action)
	}

	return nil
}
