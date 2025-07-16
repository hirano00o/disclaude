# Discord Claude システム

## プロジェクト概要

DiscordからClaude Codeを操作するシステムです。Discordのスレッド機能を活用し、各ユーザーが独立したKubernetesサンドボックス環境でClaude Codeを利用できます。

## アーキテクチャ

```
Discord User → Discord → Discord Bot (Kubernetes) → Claude Code Sandbox (Kubernetes Pod)
                                    ↓
                               PostgreSQL DB
                                    ↓
                            Kubernetes Secrets
```

## 主要機能

### 1. 認証・権限管理
- **初期認証**: 新規ユーザーに対する「オーナー確認」フロー
- **権限体系**: オーナー、一般ユーザーの2階層
- **権限管理コマンド**: `/claude add user`, `/claude add owner`, `/claude delete user`, `/claude delete owner`

### 2. セッション管理
- **スレッドベース**: 1つのDiscordスレッド = 1つのサンドボックス
- **サンドボックス制御**: `/claude start`, `/claude close`
- **リソース制限**: 最大3つのサンドボックス（環境変数で設定可能）

### 3. Claude Code 連携
- **リアルタイム通信**: Kubernetes exec APIによるWebSocket通信
- **環境セットアップ**: CLAUDE.md、カスタムコマンド、hooks の自動配置
- **ファイル管理**: 一時ディレクトリ（EmptyDir）による非永続化

## 技術スタック

- **言語**: Go
- **データベース**: PostgreSQL
- **インフラ**: Kubernetes with Cilium
- **ストレージ**: EmptyDir（一時用のみ）

## システム要件

### リソース制限
- **サンドボックス**: CPU 1GB、メモリ 2GB
- **同時実行数**: 最大3サンドボックス
- **実行時間**: 制限なし

### セキュリティ
- **Pod分離**: 完全な相互アクセス制限
- **シークレット管理**: Kubernetes Secrets
- **RBAC**: Kubernetes権限管理

## 実装計画

### Phase 1: 基盤構築
1. プロジェクト初期化とGo環境セットアップ
2. PostgreSQLデータベース設計
3. Kubernetes名前空間とRBAC設定

### Phase 2: Discord Bot 実装
1. 認証・権限管理機能
2. コマンド処理システム
3. セッション管理機能

### Phase 3: Kubernetes 連携
1. サンドボックス管理機能
2. Claude Code 通信機能
3. リソース監視・制限機能

### Phase 4: テスト・デプロイ
1. 単体テスト・統合テスト
2. Kubernetesマニフェスト作成
3. 運用ドキュメント作成

## ディレクトリ構成

```
discord-claude/
├── cmd/
│   └── main.go
├── internal/
│   ├── bot/
│   │   ├── handler.go
│   │   ├── commands.go
│   │   └── session.go
│   ├── auth/
│   │   ├── user.go
│   │   └── permission.go
│   ├── k8s/
│   │   ├── client.go
│   │   └── sandbox.go
│   ├── db/
│   │   ├── models.go
│   │   └── queries.go
│   └── config/
│       └── config.go
├── k8s/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── secret.yaml
├── sql/
│   └── schema.sql
├── go.mod
├── go.sum
├── README.md
└── CLAUDE.md
```

## 環境変数

```bash
# Discord Bot
DISCORD_TOKEN=your_discord_bot_token
DISCORD_GUILD_ID=your_guild_id

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=discord_claude
DB_PASSWORD=your_password
DB_NAME=discord_claude

# Kubernetes
KUBERNETES_NAMESPACE=discord-claude
MAX_SANDBOXES=3

# Claude Code
CLAUDE_API_KEY=your_claude_api_key
CLAUDE_CONFIG_PATH=/home/user/.claude
```

## 開発ルール

### エラーハンドリング
- すべてのエラーは`errors.Is/As`を使用してチェック
- 構造化ログを使用してエラーの追跡可能性を確保

### 並列処理
- チャネルを活用した並列処理の実装
- context.Contextを使用した適切なタイムアウト管理

### セキュリティ
- クレデンシャル情報のハードコード禁止
- 入力値の適切なサニタイズ
- SQLインジェクション対策

### テスト
- 単体テスト: `go test -race ./...`
- 統合テスト: Discord Bot とKubernetes APIの連携テスト
- 負荷テスト: 同時実行時の動作確認

## 運用考慮事項

### 監視
- Prometheus/Grafanaによるメトリクス収集（今後）
- 構造化ログによるトラブルシューティング

### 障害対応
- Discord Bot: Kubernetes Deploymentによる自動復旧
- データベース: PostgreSQL HA構成
- サンドボックス: 失敗時の自動クリーンアップ

### スケーラビリティ
- 水平スケーリング対応（レプリカ数調整）
- リソース使用量に応じた動的スケーリング