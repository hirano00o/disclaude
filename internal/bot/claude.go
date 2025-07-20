package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hirano00o/disclaude/internal/k8s"

	"github.com/sirupsen/logrus"
)

// ClaudeService はClaude Codeとの通信を管理するサービス
type ClaudeService struct {
	sandboxManager *k8s.SandboxManager
}

// NewClaudeService は新しいClaudeServiceを作成する
func NewClaudeService(sandboxManager *k8s.SandboxManager) *ClaudeService {
	return &ClaudeService{
		sandboxManager: sandboxManager,
	}
}

// SendMessage はClaude Codeにメッセージを送信し、応答を取得する
func (cs *ClaudeService) SendMessage(ctx context.Context, podName, message string) (string, error) {
	// メッセージの前処理
	processedMessage := cs.preprocessMessage(message)

	// Claude Codeコマンドの構築
	claudeCommand := fmt.Sprintf("echo %s | claude", cs.escapeShellString(processedMessage))

	// サンドボックス内でコマンド実行
	response, err := cs.sandboxManager.ExecuteCommand(ctx, podName, claudeCommand)
	if err != nil {
		return "", fmt.Errorf("failed to execute claude command: %w", err)
	}

	// 応答の後処理
	processedResponse := cs.postprocessResponse(response)

	logrus.WithFields(logrus.Fields{
		"pod_name":     podName,
		"message_len":  len(message),
		"response_len": len(processedResponse),
	}).Debug("Claude Code message processed")

	return processedResponse, nil
}

// SendFileContent はファイル内容をClaude Codeに送信する
func (cs *ClaudeService) SendFileContent(ctx context.Context, podName, filePath, content string) error {
	// ファイル作成コマンドの構築
	createFileCommand := fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF",
		cs.escapeShellString(filePath),
		content)

	// サンドボックス内でファイル作成
	_, err := cs.sandboxManager.ExecuteCommand(ctx, podName, createFileCommand)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"pod_name":    podName,
		"file_path":   filePath,
		"content_len": len(content),
	}).Debug("File created in sandbox")

	return nil
}

// GetFileContent はサンドボックスからファイル内容を取得する
func (cs *ClaudeService) GetFileContent(ctx context.Context, podName, filePath string) (string, error) {
	// ファイル読み取りコマンドの構築
	readFileCommand := fmt.Sprintf("cat %s", cs.escapeShellString(filePath))

	// サンドボックス内でファイル読み取り
	content, err := cs.sandboxManager.ExecuteCommand(ctx, podName, readFileCommand)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}

// ListFiles はサンドボックス内のファイル一覧を取得する
func (cs *ClaudeService) ListFiles(ctx context.Context, podName, directory string) (string, error) {
	// ディレクトリが指定されていない場合は現在のディレクトリを使用
	if directory == "" {
		directory = "."
	}

	// ファイル一覧取得コマンドの構築
	listCommand := fmt.Sprintf("ls -la %s", cs.escapeShellString(directory))

	// サンドボックス内でコマンド実行
	output, err := cs.sandboxManager.ExecuteCommand(ctx, podName, listCommand)
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}

	return output, nil
}

// ExecuteShellCommand はサンドボックス内で任意のシェルコマンドを実行する
func (cs *ClaudeService) ExecuteShellCommand(ctx context.Context, podName, command string) (string, error) {
	// 危険なコマンドのチェック
	if cs.isDangerousCommand(command) {
		return "", fmt.Errorf("dangerous command detected: %s", command)
	}

	// サンドボックス内でコマンド実行
	output, err := cs.sandboxManager.ExecuteCommand(ctx, podName, command)
	if err != nil {
		return "", fmt.Errorf("failed to execute shell command: %w", err)
	}

	return output, nil
}

// SetupClaudeEnvironment はClaude Code環境をセットアップする
func (cs *ClaudeService) SetupClaudeEnvironment(ctx context.Context, podName string) error {
	// CLAUDE.mdの設定
	claudeMdContent := `# Claude Code Environment

This is a temporary sandbox environment for Discord Claude integration.

## Environment Details
- CPU: 1GB
- Memory: 2GB
- Storage: Temporary (EmptyDir)
- Network: Unrestricted

## Available Tools
- Claude Code CLI
- Git
- Basic development tools
- File system access

## Notes
- All data is temporary and will be lost when the session ends
- Files are not persisted between sessions
- Use /claude close to end the session
`

	err := cs.SendFileContent(ctx, podName, "/workspace/CLAUDE.md", claudeMdContent)
	if err != nil {
		return fmt.Errorf("failed to create CLAUDE.md: %w", err)
	}

	// 作業ディレクトリの設定
	_, err = cs.sandboxManager.ExecuteCommand(ctx, podName, "cd /workspace")
	if err != nil {
		logrus.WithError(err).Warn("Failed to change to workspace directory")
	}

	// Claude Codeの初期化確認
	_, err = cs.sandboxManager.ExecuteCommand(ctx, podName, "claude --version")
	if err != nil {
		logrus.WithError(err).Warn("Claude Code CLI not available or not responding")
	}

	logrus.WithField("pod_name", podName).Info("Claude Code environment setup completed")
	return nil
}

// preprocessMessage はメッセージを前処理する
func (cs *ClaudeService) preprocessMessage(message string) string {
	// 基本的なサニタイズ
	message = strings.TrimSpace(message)

	// Discord特有のマークダウンを除去
	message = strings.ReplaceAll(message, "```", "")
	message = strings.ReplaceAll(message, "`", "'")

	// 改行の正規化
	message = strings.ReplaceAll(message, "\r\n", "\n")
	message = strings.ReplaceAll(message, "\r", "\n")

	return message
}

// postprocessResponse は応答を後処理する
func (cs *ClaudeService) postprocessResponse(response string) string {
	// 不要な制御文字を除去
	response = strings.TrimSpace(response)

	// ANSI エスケープシーケンスの除去（簡易版）
	lines := strings.Split(response, "\n")
	var cleanLines []string

	for _, line := range lines {
		// 基本的なANSIエスケープシーケンスを除去
		cleanLine := strings.TrimSpace(line)
		if cleanLine != "" {
			cleanLines = append(cleanLines, cleanLine)
		}
	}

	result := strings.Join(cleanLines, "\n")

	// Discord メッセージ長制限への対応
	const maxDiscordMessageLength = 2000
	if len(result) > maxDiscordMessageLength {
		result = result[:maxDiscordMessageLength-100] + "\n\n... (出力が長すぎるため省略されました)"
	}

	return result
}

// escapeShellString はシェル文字列をエスケープする
func (cs *ClaudeService) escapeShellString(s string) string {
	// シンプルなクォート処理
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// isDangerousCommand は危険なコマンドかどうかをチェックする
func (cs *ClaudeService) isDangerousCommand(command string) bool {
	dangerousCommands := []string{
		"rm -rf /",
		":(){ :|:& };:", // fork bomb
		"dd if=/dev/zero",
		"mkfs.",
		"fdisk",
		"cfdisk",
		"parted",
		"halt",
		"poweroff",
		"reboot",
		"shutdown",
	}

	lowerCommand := strings.ToLower(strings.TrimSpace(command))

	for _, dangerous := range dangerousCommands {
		if strings.Contains(lowerCommand, dangerous) {
			return true
		}
	}

	return false
}

// GetSandboxInfo はサンドボックスの情報を取得する
func (cs *ClaudeService) GetSandboxInfo(ctx context.Context, podName string) (*SandboxInfo, error) {
	// システム情報の取得
	systemInfo, err := cs.sandboxManager.ExecuteCommand(ctx, podName, "uname -a")
	if err != nil {
		systemInfo = "Unknown"
	}

	// ディスク使用量の取得
	diskUsage, err := cs.sandboxManager.ExecuteCommand(ctx, podName, "df -h /workspace")
	if err != nil {
		diskUsage = "Unknown"
	}

	// メモリ使用量の取得
	memoryUsage, err := cs.sandboxManager.ExecuteCommand(ctx, podName, "free -h")
	if err != nil {
		memoryUsage = "Unknown"
	}

	// 作業ディレクトリの内容
	workspaceContent, err := cs.ListFiles(ctx, podName, "/workspace")
	if err != nil {
		workspaceContent = "Unable to list files"
	}

	return &SandboxInfo{
		PodName:          podName,
		SystemInfo:       systemInfo,
		DiskUsage:        diskUsage,
		MemoryUsage:      memoryUsage,
		WorkspaceContent: workspaceContent,
		Timestamp:        time.Now(),
	}, nil
}

// SandboxInfo はサンドボックス情報を表す構造体
type SandboxInfo struct {
	PodName          string
	SystemInfo       string
	DiskUsage        string
	MemoryUsage      string
	WorkspaceContent string
	Timestamp        time.Time
}
