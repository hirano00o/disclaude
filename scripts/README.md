# Disclaude デプロイスクリプト

このディレクトリには、Disclaudeシステムのデプロイとメンテナンスのためのスクリプトが含まれています。

## 📁 スクリプト一覧

### `deploy.sh`
Disclaudeシステムを自動デプロイするスクリプトです。

**機能:**
- 前提条件のチェック
- NFSサーバー設定の確認
- シークレット設定の確認
- Kustomizeまたはkubectl apply -k を使用したデプロイ
- PostgreSQL起動の待機
- データベーススキーマ初期化の待機
- Discord Bot起動の待機
- デプロイ結果の確認

**使用方法:**
```bash
./scripts/deploy.sh
```

**前提条件:**
- kubectl がインストールされている
- Kubernetesクラスターにアクセス可能
- NFSサーバーが設定済み
- シークレット情報（Discord Token、DB Password、Claude API Key）が準備済み

### `cleanup.sh`
Disclaudeシステムを完全にクリーンアップするスクリプトです。

**機能:**
- 実行中のサンドボックスの削除
- アプリケーションの削除
- データベースの削除
- 設定とシークレットの削除
- PVC/PVの削除（オプション）
- RBAC設定の削除
- 名前空間の削除

**使用方法:**
```bash
./scripts/cleanup.sh
```

**注意事項:**
- データベースのデータが完全に削除されます
- 実行前に重要なデータのバックアップを取ってください
- PVCとPVの削除は確認プロンプトで選択可能です

## 🔧 事前準備

### 1. NFSサーバーの設定
以下のファイルでNFSサーバーの情報を更新してください：

```bash
# k8s/postgresql.yaml
spec:
  nfs:
    server: 192.168.1.100    # あなたのNFSサーバーIP
    path: /nfs/postgresql    # NFSパス

# k8s/storage-class.yaml
env:
- name: NFS_SERVER
  value: 192.168.1.100       # あなたのNFSサーバーIP
- name: NFS_PATH
  value: /nfs                # NFSルートパス
```

### 2. シークレットの設定
以下のコマンドでシークレット値をbase64エンコードしてください：

```bash
# Discord Bot Token
echo -n "your_discord_bot_token_here" | base64

# PostgreSQL Password
echo -n "your_secure_password_here" | base64

# Claude API Key
echo -n "your_claude_api_key_here" | base64
```

エンコードした値を `k8s/secret.yaml` に設定してください。

### 3. 設定ファイルの更新
`k8s/configmap.yaml` で以下を設定してください：

```yaml
data:
  discord-guild-id: "your_discord_guild_id"
  max-sandboxes: "3"  # 必要に応じて調整
```

## 📋 デプロイ手順

### 完全デプロイ
```bash
# 1. スクリプトを実行可能にする
chmod +x scripts/deploy.sh scripts/cleanup.sh

# 2. 設定ファイルを編集
vim k8s/secret.yaml
vim k8s/configmap.yaml
vim k8s/postgresql.yaml
vim k8s/storage-class.yaml

# 3. デプロイ実行
./scripts/deploy.sh
```

### 手動デプロイ
```bash
# Kustomizeを使用
kubectl apply -k k8s/

# 個別適用
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/storage-class.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/postgresql.yaml
kubectl apply -f k8s/init-schema.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

## 🔍 トラブルシューティング

### PostgreSQL起動エラー
```bash
# PostgreSQLログの確認
kubectl logs -l app=postgresql -n disclaude

# PVCの状態確認
kubectl get pvc -n disclaude

# NFSマウントの確認
kubectl describe pv postgresql-pv
```

### Discord Bot起動エラー
```bash
# Botログの確認
kubectl logs -l app=disclaude-bot -n disclaude

# 設定の確認
kubectl get configmap disclaude-config -n disclaude -o yaml
kubectl get secret disclaude-secrets -n disclaude -o yaml
```

### スキーマ初期化エラー
```bash
# 初期化ジョブログの確認
kubectl logs job/postgresql-schema-init -n disclaude

# データベース接続テスト
kubectl exec -it deployment/postgresql -n disclaude -- psql -U discord_claude -d discord_claude -c "SELECT version();"
```

## 🚨 緊急時の対応

### 全システム停止
```bash
# 緊急停止
kubectl delete deployment disclaude-bot -n disclaude
kubectl delete pods -l app=claude-sandbox -n disclaude --force --grace-period=0

# 完全クリーンアップ
./scripts/cleanup.sh
```

### データベース復旧
```bash
# PostgreSQL再起動
kubectl rollout restart deployment/postgresql -n disclaude

# スキーマ再初期化
kubectl delete job postgresql-schema-init -n disclaude
kubectl apply -f k8s/init-schema.yaml
```

## 📊 運用監視

### 基本監視コマンド
```bash
# システム状態
kubectl get pods,svc,pvc -n disclaude

# リソース使用量
kubectl top pods -n disclaude

# イベント確認
kubectl get events -n disclaude --sort-by=.metadata.creationTimestamp
```

### ログ監視
```bash
# Bot リアルタイムログ
kubectl logs -l app=disclaude-bot -n disclaude -f

# PostgreSQL ログ
kubectl logs -l app=postgresql -n disclaude -f

# すべてのログ
kubectl logs -l component=disclaude -n disclaude -f
```