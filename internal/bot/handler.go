package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hirano00o/disclaude/internal/auth"
	"github.com/hirano00o/disclaude/internal/config"
	"github.com/hirano00o/disclaude/internal/db"
	"github.com/hirano00o/disclaude/internal/k8s"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// Bot はDiscord Botの主要構造体
type Bot struct {
	session        *discordgo.Session
	config         *config.Config
	db             *db.DB
	userService    *auth.UserService
	permService    *auth.PermissionService
	k8sClient      *k8s.Client
	sandboxManager *k8s.SandboxManager
	claudeService  *ClaudeService
}

// New は新しいBotインスタンスを作成する
func New(cfg *config.Config, database *db.DB) (*Bot, error) {
	// Discord セッションの作成
	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	// Kubernetes クライアントの初期化
	k8sClient, err := k8s.NewClient(cfg.Kubernetes.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// サービスの初期化
	userService := auth.NewUserService(database)
	permService := auth.NewPermissionService(database)
	sandboxManager := k8s.NewSandboxManager(k8sClient, database, cfg)
	claudeService := NewClaudeService(sandboxManager)

	bot := &Bot{
		session:        session,
		config:         cfg,
		db:             database,
		userService:    userService,
		permService:    permService,
		k8sClient:      k8sClient,
		sandboxManager: sandboxManager,
		claudeService:  claudeService,
	}

	// イベントハンドラーの登録
	session.AddHandler(bot.messageHandler)
	session.AddHandler(bot.readyHandler)

	return bot, nil
}

// Start はBotを開始する
func (b *Bot) Start(ctx context.Context) error {
	// Kubernetes名前空間の作成
	if err := b.k8sClient.CreateNamespace(ctx); err != nil {
		return fmt.Errorf("failed to create kubernetes namespace: %w", err)
	}

	// Discord接続を開く
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open discord session: %w", err)
	}

	logrus.Info("Discord bot started successfully")
	return nil
}

// Stop はBotを停止する
func (b *Bot) Stop() {
	if b.session != nil {
		b.session.Close()
	}
}

// readyHandler はBot準備完了時のハンドラー
func (b *Bot) readyHandler(s *discordgo.Session, event *discordgo.Ready) {
	logrus.WithField("username", event.User.Username).Info("Bot is ready and logged in")
	
	// Botのステータス設定
	err := s.UpdateGameStatus(0, "Claude Code サポート中")
	if err != nil {
		logrus.WithError(err).Error("Failed to set bot status")
	}
}

// messageHandler はメッセージ受信時のハンドラー
func (b *Bot) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Bot自身のメッセージは無視
	if m.Author.ID == s.State.User.ID {
		return
	}

	// DMまたはコマンドのチェック
	if m.GuildID == "" || strings.HasPrefix(m.Content, "/claude") {
		b.handleCommand(s, m)
		return
	}

	// スレッド内のメッセージかチェック
	if m.ChannelID != "" {
		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			logrus.WithError(err).Error("Failed to get channel info")
			return
		}

		// スレッドの場合、Claude Codeとの通信を処理
		if channel.Type == discordgo.ChannelTypeGuildPublicThread || channel.Type == discordgo.ChannelTypeGuildPrivateThread {
			b.handleThreadMessage(s, m)
		}
	}
}

// handleCommand はコマンドを処理する
func (b *Bot) handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	content := strings.TrimSpace(m.Content)
	parts := strings.Fields(content)
	
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "/claude") {
		return
	}

	// 初回ユーザーの場合、認証フローを開始
	user, err := b.userService.GetUser(m.Author.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get user")
		b.sendErrorMessage(s, m.ChannelID, "ユーザー情報の取得に失敗しました")
		return
	}

	if user == nil {
		b.handleInitialAuthentication(s, m)
		return
	}

	// コマンドの解析と実行
	if len(parts) < 2 {
		b.sendHelpMessage(s, m.ChannelID)
		return
	}

	subcommand := parts[1]
	switch subcommand {
	case "start":
		b.handleStartCommand(s, m, user)
	case "close":
		b.handleCloseCommand(s, m, user)
	case "add":
		if len(parts) >= 4 {
			b.handleAddCommand(s, m, user, parts[2], parts[3])
		} else {
			b.sendErrorMessage(s, m.ChannelID, "使用方法: `/claude add user <ユーザーID>` または `/claude add owner <ユーザーID>`")
		}
	case "delete":
		if len(parts) >= 4 {
			b.handleDeleteCommand(s, m, user, parts[2], parts[3])
		} else {
			b.sendErrorMessage(s, m.ChannelID, "使用方法: `/claude delete user <ユーザーID>` または `/claude delete owner <ユーザーID>`")
		}
	case "status":
		b.handleStatusCommand(s, m, user)
	case "help":
		b.sendHelpMessage(s, m.ChannelID)
	default:
		b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("不明なコマンド: `%s`", subcommand))
	}
}

// handleInitialAuthentication は初回認証を処理する
func (b *Bot) handleInitialAuthentication(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 既存の認証プロセスをチェック
	content := strings.ToLower(strings.TrimSpace(m.Content))
	
	if content == "yes" || content == "y" || content == "はい" {
		// オーナーとして登録
		user, err := b.userService.InitializeUser(m.Author.ID, m.Author.Username, true)
		if err != nil {
			logrus.WithError(err).Error("Failed to initialize owner")
			b.sendErrorMessage(s, m.ChannelID, "オーナー登録に失敗しました")
			return
		}
		
		b.sendMessage(s, m.ChannelID, fmt.Sprintf("✅ オーナーとして登録されました、%sさん！\nClaude Codeサンドボックスを利用できます。\n\n使用方法: `/claude help`", user.Username))
	
	} else if content == "no" || content == "n" || content == "いいえ" {
		// 一般ユーザーとして終了
		b.sendMessage(s, m.ChannelID, "オーナー登録をキャンセルしました。Claude Codeサンドボックスを利用するには、オーナーからユーザー追加をしてもらってください。")
	
	} else if strings.HasPrefix(content, "/claude") {
		// 初回コマンド - オーナー確認
		b.sendMessage(s, m.ChannelID, fmt.Sprintf("こんにちは %sさん！\n\n🤖 **あなたが私のオーナーですか？**\n\n✅ オーナーの場合: `Yes` と返信\n❌ オーナーでない場合: `No` と返信\n\n※オーナーはユーザー管理権限を持ちます", m.Author.Username))
	}
}

// handleThreadMessage はスレッド内のメッセージを処理する
func (b *Bot) handleThreadMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// スレッドIDでセッションを取得
	session, err := b.db.GetSessionByThreadID(m.ChannelID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get session")
		return
	}

	if session == nil || !session.IsActive() {
		return
	}

	// ユーザー権限チェック
	user, err := b.userService.GetUser(m.Author.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get user")
		return
	}

	if user == nil {
		b.sendErrorMessage(s, m.ChannelID, "ユーザーが登録されていません")
		return
	}

	// セッションの所有者またはオーナーのみ操作可能
	canUse, err := b.permService.CanDeleteSandbox(m.Author.ID, session.UserID)
	if err != nil {
		logrus.WithError(err).Error("Failed to check permission")
		return
	}

	if !canUse {
		b.sendErrorMessage(s, m.ChannelID, "このセッションを使用する権限がありません")
		return
	}

	// Claude Codeにメッセージを送信
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := b.claudeService.SendMessage(ctx, session.SandboxName, m.Content)
	if err != nil {
		logrus.WithError(err).Error("Failed to send message to Claude Code")
		b.sendErrorMessage(s, m.ChannelID, "Claude Codeとの通信に失敗しました")
		return
	}

	// 応答を送信
	b.sendMessage(s, m.ChannelID, response)
}

// sendMessage はメッセージを送信する
func (b *Bot) sendMessage(s *discordgo.Session, channelID, content string) {
	// 長いメッセージの分割処理
	const maxLength = 2000
	if len(content) <= maxLength {
		_, err := s.ChannelMessageSend(channelID, content)
		if err != nil {
			logrus.WithError(err).Error("Failed to send message")
		}
		return
	}

	// メッセージを分割して送信
	for len(content) > 0 {
		end := maxLength
		if end > len(content) {
			end = len(content)
		}
		
		// コードブロックやマークダウンを考慮した分割ポイントを探す
		chunk := content[:end]
		if end < len(content) {
			// 適切な分割ポイントを探す
			if lastNewline := strings.LastIndex(chunk, "\n"); lastNewline > maxLength/2 {
				end = lastNewline + 1
				chunk = content[:end]
			}
		}

		_, err := s.ChannelMessageSend(channelID, chunk)
		if err != nil {
			logrus.WithError(err).Error("Failed to send message chunk")
		}

		content = content[end:]
		time.Sleep(100 * time.Millisecond) // Rate limit対策
	}
}

// sendErrorMessage はエラーメッセージを送信する
func (b *Bot) sendErrorMessage(s *discordgo.Session, channelID, message string) {
	errorMsg := fmt.Sprintf("❌ **エラー**\n%s", message)
	b.sendMessage(s, channelID, errorMsg)
}

// sendHelpMessage はヘルプメッセージを送信する
func (b *Bot) sendHelpMessage(s *discordgo.Session, channelID string) {
	helpMessage := `🤖 **Claude Code Bot - コマンド一覧**

**基本コマンド:**
• `+"`/claude start`"+` - 新しいClaude Codeセッションを開始
• `+"`/claude close`"+` - 現在のセッションを終了
• `+"`/claude status`"+` - 現在のセッション状況を確認
• `+"`/claude help`"+` - このヘルプを表示

**オーナー専用コマンド:**
• `+"`/claude add user <ユーザーID>`"+` - ユーザーを追加
• `+"`/claude add owner <ユーザーID>`"+` - ユーザーをオーナーに昇格
• `+"`/claude delete user <ユーザーID>`"+` - ユーザーを削除
• `+"`/claude delete owner <ユーザーID>`"+` - オーナーを一般ユーザーに降格

**使用方法:**
1. `+"`/claude start`"+` でスレッドを作成し、Claude Codeセッションを開始
2. スレッド内でClaude Codeと自由に会話
3. 作業完了後は `+"`/claude close`"+` でセッション終了

**注意事項:**
• 同時に作成できるサンドボックスは最大3つまで
• セッションは作成者のみ終了可能（オーナーは例外）
• ファイルは一時的なもので、セッション終了時に削除されます`

	b.sendMessage(s, channelID, helpMessage)
}
