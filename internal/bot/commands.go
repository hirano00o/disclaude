package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"discord-claude/internal/db"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// handleStartCommand は `/claude start` コマンドを処理する
func (b *Bot) handleStartCommand(s *discordgo.Session, m *discordgo.MessageCreate, user *db.User) {
	// 権限チェック
	if err := b.permService.ValidateUserAction(user.DiscordID, "create_sandbox"); err != nil {
		b.sendErrorMessage(s, m.ChannelID, err.Error())
		return
	}

	// 既存のアクティブセッションチェック
	existingSession, err := b.db.GetSessionByThreadID(m.ChannelID)
	if err != nil {
		logrus.WithError(err).Error("Failed to check existing session")
		b.sendErrorMessage(s, m.ChannelID, "セッション確認中にエラーが発生しました")
		return
	}

	if existingSession != nil && existingSession.IsActive() {
		b.sendErrorMessage(s, m.ChannelID, "このチャンネルには既にアクティブなセッションが存在します")
		return
	}

	// スレッドの作成
	thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, &discordgo.ThreadStart{
		Name: fmt.Sprintf("Claude Code - %s", user.Username),
		Type: discordgo.ChannelTypeGuildPublicThread,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed to create thread")
		b.sendErrorMessage(s, m.ChannelID, "スレッドの作成に失敗しました")
		return
	}

	// セッションの作成
	sandboxName := fmt.Sprintf("claude-sandbox-%s", strings.ReplaceAll(thread.ID, "_", "-"))
	session, err := b.db.CreateSession(user.ID, thread.ID, sandboxName)
	if err != nil {
		logrus.WithError(err).Error("Failed to create session")
		b.sendErrorMessage(s, thread.ID, "セッションの作成に失敗しました")
		return
	}

	// サンドボックスの作成
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	b.sendMessage(s, thread.ID, "🚀 **Claude Codeサンドボックスを作成中...**\n少々お待ちください。")

	sandbox, err := b.sandboxManager.CreateSandbox(ctx, session.ID, thread.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed to create sandbox")
		b.sendErrorMessage(s, thread.ID, fmt.Sprintf("サンドボックスの作成に失敗しました: %v", err))
		// セッションステータスを失敗に更新
		b.db.UpdateSessionStatus(session.ID, "failed")
		return
	}

	// サンドボックスの準備完了まで待機
	b.sendMessage(s, thread.ID, "⏳ **サンドボックスの準備中...**")

	err = b.sandboxManager.WaitForSandboxReady(ctx, sandbox.PodName, 3*time.Minute)
	if err != nil {
		logrus.WithError(err).Error("Failed to wait for sandbox ready")
		b.sendErrorMessage(s, thread.ID, "サンドボックスの準備に失敗しました")
		return
	}

	// 成功メッセージの送信
	successMessage := fmt.Sprintf(`✅ **Claude Codeサンドボックスが準備完了しました！**

🏷️ **セッション情報:**
• セッションID: %d
• サンドボックス名: %s
• 作成者: %s
• CPU: 1GB, メモリ: 2GB

💬 **使用方法:**
このスレッド内でClaude Codeと自由に会話できます。
セッション終了時は \`/claude close\` を実行してください。

🔧 **利用可能な機能:**
• ファイル作成・編集
• コード実行とテスト
• プロジェクト管理
• Git操作

⚠️ **注意事項:**
• ファイルは一時的なものです
• セッション終了時にすべてのデータが削除されます`, 
		session.ID, 
		sandbox.PodName, 
		user.Username)

	b.sendMessage(s, thread.ID, successMessage)

	logrus.WithFields(logrus.Fields{
		"user_id":      user.ID,
		"session_id":   session.ID,
		"thread_id":    thread.ID,
		"sandbox_name": sandbox.PodName,
	}).Info("Claude Code session started successfully")
}

// handleCloseCommand は `/claude close` コマンドを処理する
func (b *Bot) handleCloseCommand(s *discordgo.Session, m *discordgo.MessageCreate, user *db.User) {
	// 権限チェック
	if err := b.permService.ValidateUserAction(user.DiscordID, "close_sandbox"); err != nil {
		b.sendErrorMessage(s, m.ChannelID, err.Error())
		return
	}

	// セッションの取得
	session, err := b.db.GetSessionByThreadID(m.ChannelID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get session")
		b.sendErrorMessage(s, m.ChannelID, "セッション情報の取得に失敗しました")
		return
	}

	if session == nil {
		b.sendErrorMessage(s, m.ChannelID, "このチャンネルにはアクティブなセッションが存在しません")
		return
	}

	if !session.IsActive() {
		b.sendErrorMessage(s, m.ChannelID, "このセッションは既に終了しています")
		return
	}

	// セッション所有者またはオーナーのみ終了可能
	canClose, err := b.permService.CanDeleteSandbox(user.DiscordID, session.UserID)
	if err != nil {
		logrus.WithError(err).Error("Failed to check permission")
		b.sendErrorMessage(s, m.ChannelID, "権限確認中にエラーが発生しました")
		return
	}

	if !canClose {
		b.sendErrorMessage(s, m.ChannelID, "このセッションを終了する権限がありません")
		return
	}

	// サンドボックスの削除
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b.sendMessage(s, m.ChannelID, "🛑 **セッションを終了中...**")

	err = b.sandboxManager.DeleteSandbox(ctx, session.SandboxName)
	if err != nil {
		logrus.WithError(err).Error("Failed to delete sandbox")
		b.sendErrorMessage(s, m.ChannelID, "サンドボックスの削除に失敗しました")
		return
	}

	// セッションステータスの更新
	err = b.db.UpdateSessionStatus(session.ID, "terminated")
	if err != nil {
		logrus.WithError(err).Error("Failed to update session status")
	}

	// 終了メッセージの送信
	endMessage := fmt.Sprintf(`✅ **セッションが正常に終了しました**

🏷️ **終了したセッション:**
• セッションID: %d
• サンドボックス名: %s
• 実行時間: %s

💾 **データについて:**
このセッションで作成されたファイルやデータはすべて削除されました。

🆕 **新しいセッション:**
新しいセッションを開始するには \`/claude start\` を実行してください。`,
		session.ID,
		session.SandboxName,
		time.Since(session.CreatedAt).Round(time.Second).String())

	b.sendMessage(s, m.ChannelID, endMessage)

	logrus.WithFields(logrus.Fields{
		"user_id":      user.ID,
		"session_id":   session.ID,
		"thread_id":    m.ChannelID,
		"sandbox_name": session.SandboxName,
		"duration":     time.Since(session.CreatedAt),
	}).Info("Claude Code session terminated successfully")
}

// handleAddCommand は `/claude add` コマンドを処理する
func (b *Bot) handleAddCommand(s *discordgo.Session, m *discordgo.MessageCreate, user *db.User, target, userID string) {
	// 権限チェック
	if err := b.permService.ValidateUserAction(user.DiscordID, "add_user"); err != nil {
		b.sendErrorMessage(s, m.ChannelID, err.Error())
		return
	}

	// ユーザーIDの形式チェック（Discordのsnowflake形式）
	if len(userID) < 15 || len(userID) > 20 {
		b.sendErrorMessage(s, m.ChannelID, "無効なユーザーIDです")
		return
	}

	// 対象ユーザーの情報取得
	targetUser, err := s.User(userID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get target user from Discord")
		b.sendErrorMessage(s, m.ChannelID, "指定されたユーザーが見つかりません")
		return
	}

	switch target {
	case "user":
		// 一般ユーザーとして追加
		createdUser, err := b.userService.AddUser(user.DiscordID, targetUser.ID, targetUser.Username)
		if err != nil {
			logrus.WithError(err).Error("Failed to add user")
			b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("ユーザー追加に失敗しました: %v", err))
			return
		}

		successMessage := fmt.Sprintf(`✅ **ユーザーを追加しました**

👤 **追加されたユーザー:**
• 名前: %s
• ID: %s
• 権限: 一般ユーザー
• 追加者: %s

🎯 **権限:**
• Claude Codeサンドボックスの作成・使用
• 自分のセッションの管理`,
			createdUser.Username,
			createdUser.DiscordID,
			user.Username)

		b.sendMessage(s, m.ChannelID, successMessage)

		logrus.WithFields(logrus.Fields{
			"requester_id": user.ID,
			"target_id":    createdUser.ID,
			"target_role":  "user",
		}).Info("User added successfully")

	case "owner":
		// オーナーに昇格
		err := b.userService.PromoteToOwner(user.DiscordID, targetUser.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to promote to owner")
			b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("オーナー昇格に失敗しました: %v", err))
			return
		}

		successMessage := fmt.Sprintf(`✅ **ユーザーをオーナーに昇格しました**

👑 **昇格されたオーナー:**
• 名前: %s
• ID: %s
• 権限: オーナー
• 昇格者: %s

🎯 **オーナー権限:**
• Claude Codeサンドボックスの作成・使用
• 全セッションの管理
• ユーザー管理（追加・削除・権限変更）`,
			targetUser.Username,
			targetUser.ID,
			user.Username)

		b.sendMessage(s, m.ChannelID, successMessage)

		logrus.WithFields(logrus.Fields{
			"requester_id": user.ID,
			"target_id":    targetUser.ID,
			"action":       "promote_to_owner",
		}).Info("User promoted to owner successfully")

	default:
		b.sendErrorMessage(s, m.ChannelID, "無効なターゲットです。`user` または `owner` を指定してください")
	}
}

// handleDeleteCommand は `/claude delete` コマンドを処理する
func (b *Bot) handleDeleteCommand(s *discordgo.Session, m *discordgo.MessageCreate, user *db.User, target, userID string) {
	// 権限チェック
	if err := b.permService.ValidateUserAction(user.DiscordID, "delete_user"); err != nil {
		b.sendErrorMessage(s, m.ChannelID, err.Error())
		return
	}

	// ユーザーIDの形式チェック
	if len(userID) < 15 || len(userID) > 20 {
		b.sendErrorMessage(s, m.ChannelID, "無効なユーザーIDです")
		return
	}

	// 対象ユーザーの情報取得
	targetUser, err := b.userService.GetUser(userID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get target user")
		b.sendErrorMessage(s, m.ChannelID, "ユーザー情報の取得に失敗しました")
		return
	}

	if targetUser == nil {
		b.sendErrorMessage(s, m.ChannelID, "指定されたユーザーは登録されていません")
		return
	}

	switch target {
	case "user":
		// ユーザーを削除
		err := b.userService.RemoveUser(user.DiscordID, userID)
		if err != nil {
			logrus.WithError(err).Error("Failed to remove user")
			b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("ユーザー削除に失敗しました: %v", err))
			return
		}

		successMessage := fmt.Sprintf(`✅ **ユーザーを削除しました**

👤 **削除されたユーザー:**
• 名前: %s
• ID: %s
• 削除者: %s

⚠️ **注意:**
このユーザーのアクティブなセッションも終了されます。`,
			targetUser.Username,
			targetUser.DiscordID,
			user.Username)

		b.sendMessage(s, m.ChannelID, successMessage)

		logrus.WithFields(logrus.Fields{
			"requester_id": user.ID,
			"target_id":    targetUser.ID,
			"action":       "remove_user",
		}).Info("User removed successfully")

	case "owner":
		// オーナーを一般ユーザーに降格
		err := b.userService.DemoteFromOwner(user.DiscordID, userID)
		if err != nil {
			logrus.WithError(err).Error("Failed to demote from owner")
			b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("オーナー降格に失敗しました: %v", err))
			return
		}

		successMessage := fmt.Sprintf(`✅ **オーナーを一般ユーザーに降格しました**

👤 **降格されたユーザー:**
• 名前: %s
• ID: %s
• 新しい権限: 一般ユーザー
• 実行者: %s

🎯 **現在の権限:**
• Claude Codeサンドボックスの作成・使用
• 自分のセッションの管理`,
			targetUser.Username,
			targetUser.DiscordID,
			user.Username)

		b.sendMessage(s, m.ChannelID, successMessage)

		logrus.WithFields(logrus.Fields{
			"requester_id": user.ID,
			"target_id":    targetUser.ID,
			"action":       "demote_from_owner",
		}).Info("User demoted from owner successfully")

	default:
		b.sendErrorMessage(s, m.ChannelID, "無効なターゲットです。`user` または `owner` を指定してください")
	}
}

// handleStatusCommand は `/claude status` コマンドを処理する
func (b *Bot) handleStatusCommand(s *discordgo.Session, m *discordgo.MessageCreate, user *db.User) {
	// サンドボックス使用状況の取得
	usage, err := b.db.GetSandboxUsage()
	if err != nil {
		logrus.WithError(err).Error("Failed to get sandbox usage")
		b.sendErrorMessage(s, m.ChannelID, "ステータス情報の取得に失敗しました")
		return
	}

	// 現在のセッションチェック
	currentSession, err := b.db.GetSessionByThreadID(m.ChannelID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get current session")
	}

	// Kubernetesサンドボックス一覧の取得
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sandboxes, err := b.sandboxManager.ListSandboxes(ctx)
	if err != nil {
		logrus.WithError(err).Error("Failed to list sandboxes")
	}

	// ステータスメッセージの作成
	statusMessage := fmt.Sprintf(`📊 **Discord Claude システム ステータス**

👤 **ユーザー情報:**
• 名前: %s
• 権限: %s

🏗️ **サンドボックス使用状況:**
• 使用中: %d/%d
• 利用可能: %d

📍 **現在のチャンネル:**`,
		user.Username,
		user.Role,
		usage.CurrentCount,
		usage.MaxCount,
		usage.RemainingCapacity())

	if currentSession != nil && currentSession.IsActive() {
		sessionDuration := time.Since(currentSession.CreatedAt).Round(time.Second)
		statusMessage += fmt.Sprintf(`
• セッション: アクティブ ✅
• セッションID: %d
• 実行時間: %s
• サンドボックス: %s`,
			currentSession.ID,
			sessionDuration.String(),
			currentSession.SandboxName)
	} else {
		statusMessage += "\n• セッション: なし ⭕"
	}

	// アクティブなサンドボックス一覧
	if len(sandboxes) > 0 {
		statusMessage += "\n\n🔧 **アクティブなサンドボックス一覧:**"
		for i, sandbox := range sandboxes {
			if i >= 10 { // 最大10個まで表示
				statusMessage += fmt.Sprintf("\n• ... 他 %d 個", len(sandboxes)-10)
				break
			}
			statusMessage += fmt.Sprintf("\n• %s (%s)", sandbox.Name, sandbox.Status.Phase)
		}
	}

	// 権限情報
	if user.IsOwner() {
		statusMessage += "\n\n👑 **オーナー権限で利用可能なコマンド:**\n• `/claude add user <ID>` - ユーザー追加\n• `/claude add owner <ID>` - オーナー昇格\n• `/claude delete user <ID>` - ユーザー削除\n• `/claude delete owner <ID>` - オーナー降格"
	}

	b.sendMessage(s, m.ChannelID, statusMessage)
}