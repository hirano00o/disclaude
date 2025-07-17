# Discord Claude

DiscordからClaude Codeを操作するためのシステムです。Discordのスレッド機能を活用し、各ユーザーが独立したKubernetesサンドボックス環境でClaude Codeを利用できます。

## 🎯 概要

- **Discord統合**: Discord Botを通じてClaude Codeを操作
- **スレッドベース**: 1つのDiscordスレッド = 1つのサンドボックス環境
- **Kubernetes基盤**: スケーラブルで分離されたサンドボックス環境
- **権限管理**: オーナー/ユーザーの2階層権限システム
- **リソース制限**: 最大3つのサンドボックス同時実行

## 🏗️ アーキテクチャ

```
Discord User → Discord → Discord Bot (Kubernetes) → Claude Code Sandbox (Kubernetes Pod)
                                    ↓
                               PostgreSQL DB
                                    ↓
                            Kubernetes Secrets
```

## 🚀 主要機能

### Discord Bot コマンド

- `/claude start` - 新しいClaude Codeセッションを開始
- `/claude close` - 現在のセッションを終了
- `/claude status` - 現在のセッション状況を確認
- `/claude help` - ヘルプを表示

### オーナー専用コマンド

- `/claude add user <ユーザーID>` - ユーザーを追加
- `/claude add owner <ユーザーID>` - ユーザーをオーナーに昇格
- `/claude delete user <ユーザーID>` - ユーザーを削除
- `/claude delete owner <ユーザーID>` - オーナーを一般ユーザーに降格

### 認証システム

- 初回利用時の「オーナー確認」フロー
- オーナー/一般ユーザーの権限管理
- 自分自身の削除・降格防止

## 📋 要件

### システム要件

- **Kubernetes クラスター**: バージョン 1.25+
- **PostgreSQL**: バージョン 13+
- **Go**: バージョン 1.24+
- **Discord Bot Token**: Discord Developer Portalで取得

### リソース要件

- **サンドボックス**: CPU 1GB、メモリ 2GB
- **Bot**: CPU 500m、メモリ 512Mi
- **同時実行数**: 最大3サンドボックス（設定可能）

## 🛠️ セットアップ

### Kubernetes環境での運用

本システムはKubernetes環境での本格運用を想定しています。PostgreSQLもKubernetesクラスター内で動作し、NFSによるデータ永続化を行います。

### ローカル開発環境

ローカル開発時は、PostgreSQLをDockerで起動し、Botのみローカルで実行することも可能です。

### 1. 事前準備

```bash
# リポジトリのクローン
git clone <repository-url>
cd discord-claude

# Go モジュールの初期化
go mod download
```

### 2. Discord Bot の作成

1. [Discord Developer Portal](https://discord.com/developers/applications)でアプリケーションを作成
2. Bot トークンを取得
3. 必要な権限を設定:
   - Send Messages
   - Create Public Threads
   - Use Slash Commands

### 3. 環境変数の設定

```bash
# .env ファイルを作成
cp .env.example .env

# 必要な値を設定
DISCORD_TOKEN=your_discord_bot_token
DISCORD_GUILD_ID=your_guild_id
DB_HOST=localhost
DB_PORT=5432
DB_USER=discord_claude
DB_PASSWORD=your_password
DB_NAME=discord_claude
CLAUDE_API_KEY=your_claude_api_key
KUBERNETES_NAMESPACE=discord-claude
MAX_SANDBOXES=3
```

### 4. データベースの準備

```bash
# Kubernetes環境の場合：
# PostgreSQLはKubernetesクラスター内で自動的にデプロイされます

# ローカル開発環境の場合：
docker run -d \
  --name discord-claude-db \
  -e POSTGRES_USER=discord_claude \
  -e POSTGRES_PASSWORD=your_password \
  -e POSTGRES_DB=discord_claude \
  -p 5432:5432 \
  postgres:15

# マイグレーションの実行（ローカル開発時）
go run cmd/main.go
```

### 5. NFSサーバーの設定

```bash
# NFSサーバーの設定（事前に準備）
# 以下のファイルでNFSサーバーの情報を更新してください：
# - k8s/postgresql.yaml: NFS サーバーIP とパス
# - k8s/storage-class.yaml: NFS サーバーIP とパス

# 例：
# server: 192.168.1.100
# path: /nfs/postgresql
```

### 6. Kubernetesデプロイ

```bash
# シークレットの作成（base64エンコードが必要）
echo -n "your_discord_token" | base64  # Discord Token
echo -n "your_db_password" | base64    # DB Password
echo -n "your_claude_api_key" | base64 # Claude API Key

# シークレットファイルを編集してエンコード済みの値を設定
vim k8s/secret.yaml

# 設定ファイルを編集（Discord Guild ID など）
vim k8s/configmap.yaml

# 自動デプロイスクリプトの実行
./scripts/deploy.sh

# または手動デプロイ
kubectl apply -k k8s/
```

### 7. Dockerイメージのビルド

```bash
# Dockerfileの作成
cat <<EOF > Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o discord-claude cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/discord-claude .
COPY --from=builder /app/sql ./sql

CMD ["./discord-claude"]
EOF

# イメージのビルドとプッシュ
docker build -t discord-claude:latest .
docker tag discord-claude:latest your-registry/discord-claude:latest
docker push your-registry/discord-claude:latest
```

## 📖 使用方法

### 1. 初回セットアップ

1. Discord サーバーでBot にDMまたは`/claude`コマンドを送信
2. 「あなたが私のオーナーですか？」の質問に回答
   - `Yes`: オーナーとして登録
   - `No`: 登録をキャンセル

### 2. セッションの開始

1. `/claude start`コマンドでスレッド作成
2. サンドボックスの準備完了を待機
3. スレッド内でClaude Codeと自由に会話

### 3. セッションの終了

1. `/claude close`コマンドでセッション終了
2. すべてのデータが削除される

### 4. ユーザー管理（オーナーのみ）

```bash
# ユーザー追加
/claude add user 123456789012345678

# オーナー昇格
/claude add owner 123456789012345678

# ユーザー削除
/claude delete user 123456789012345678

# オーナー降格
/claude delete owner 123456789012345678
```

## 🔧 開発

### ローカル開発

```bash
# 依存関係のインストール
go mod download

# テストの実行
go test -race ./...

# リンターの実行
golangci-lint run

# フォーマットの実行
go fmt ./...

# 脆弱性チェック
govulncheck ./...
```

### テスト環境

```bash
# テスト用データベースの準備
docker run -d \
  --name discord-claude-test-db \
  -e POSTGRES_USER=test_user \
  -e POSTGRES_PASSWORD=test_password \
  -e POSTGRES_DB=test_discord_claude \
  -p 5433:5432 \
  postgres:13

# テストの実行
DB_HOST=localhost DB_PORT=5433 DB_USER=test_user DB_PASSWORD=test_password DB_NAME=test_discord_claude go test ./...
```

## 📁 プロジェクト構造

```
discord-claude/
├── cmd/
│   └── main.go                 # エントリーポイント
├── internal/
│   ├── auth/                   # 認証・権限管理
│   │   ├── user.go
│   │   ├── permission.go
│   │   └── user_test.go
│   ├── bot/                    # Discord Bot
│   │   ├── handler.go
│   │   ├── commands.go
│   │   ├── session.go
│   │   └── claude.go
│   ├── config/                 # 設定管理
│   │   └── config.go
│   ├── db/                     # データベース
│   │   ├── models.go
│   │   ├── queries.go
│   │   └── queries_test.go
│   └── k8s/                    # Kubernetes
│       ├── client.go
│       └── sandbox.go
├── k8s/                        # Kubernetesマニフェスト
│   ├── namespace.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── rbac.yaml
│   ├── postgresql.yaml         # PostgreSQL設定
│   ├── storage-class.yaml      # NFS StorageClass
│   ├── init-schema.yaml        # DB初期化
│   ├── kustomization.yaml      # Kustomize設定
│   └── replica-patch.yaml      # レプリカ設定
├── scripts/
│   ├── deploy.sh               # デプロイスクリプト
│   └── cleanup.sh              # クリーンアップスクリプト
├── sql/
│   └── schema.sql              # データベーススキーマ
├── go.mod
├── go.sum
├── CLAUDE.md                   # プロジェクト計画
└── README.md
```

## ⚠️ 注意事項

### セキュリティ

- シークレット情報はKubernetes Secretsで管理
- サンドボックス間は完全に分離
- ネットワークポリシーによる通信制限
- 入力値のサニタイズ実装

### 制限事項

- サンドボックス内のファイルは一時的（セッション終了時に削除）
- 同時実行サンドボックス数に制限あり（デフォルト3つ）
- リソース使用量の制限あり
- PostgreSQLデータは永続化されるが、適切なバックアップが必要

### パフォーマンス

- サンドボックス起動に1-3分程度要する場合あり
- 大量の出力は自動的にトリミング
- Discord API レート制限を考慮

## 🤝 コントリビューション

1. フォークする
2. フィーチャーブランチを作成 (`git checkout -b feature/amazing-feature`)
3. 変更をコミット (`git commit -m 'Add amazing feature'`)
4. ブランチをプッシュ (`git push origin feature/amazing-feature`)
5. プルリクエストを作成

## 📄 ライセンス

このプロジェクトはMITライセンスの下で公開されています。詳細は[LICENSE](LICENSE)ファイルを参照してください。

## 📞 サポート

- 問題や質問は[Issues](https://github.com/your-repo/discord-claude/issues)で報告
- ドキュメントは[CLAUDE.md](CLAUDE.md)を参照
- 開発者向け情報は各パッケージのコメントを参照

## 🚀 運用コマンド

```bash
# デプロイ
./scripts/deploy.sh

# クリーンアップ
./scripts/cleanup.sh

# ログ確認
kubectl logs -l app=discord-claude-bot -n discord-claude -f

# データベース接続
kubectl exec -it deployment/postgresql -n discord-claude -- psql -U discord_claude -d discord_claude

# システム状態確認
kubectl get pods,svc,pvc -n discord-claude
```