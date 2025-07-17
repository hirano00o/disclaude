#!/bin/bash

# Discord Claude システムのデプロイスクリプト
set -e

NAMESPACE="discord-claude"
KUSTOMIZE_DIR="k8s"

echo "🚀 Discord Claude システムのデプロイを開始します..."

# 前提条件のチェック
echo "📋 前提条件をチェック中..."

# kubectlのチェック
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl がインストールされていません"
    exit 1
fi

# kustomizeのチェック
if ! command -v kustomize &> /dev/null; then
    echo "⚠️  kustomize がインストールされていません。kubectl apply -k を使用します"
    USE_KUSTOMIZE=false
else
    USE_KUSTOMIZE=true
fi

# NFSサーバーの設定確認
echo "🔧 NFSサーバーの設定を確認してください："
echo "   - k8s/postgresql.yaml の NFS サーバーIP"
echo "   - k8s/storage-class.yaml の NFS サーバーIP"
echo ""
read -p "NFSサーバーの設定は完了していますか？ (y/N): " confirm
if [[ ! $confirm =~ ^[Yy]$ ]]; then
    echo "❌ NFSサーバーの設定を完了してからデプロイしてください"
    exit 1
fi

# シークレットの設定確認
echo "🔐 シークレットの設定を確認してください："
echo "   - Discord Bot Token"
echo "   - PostgreSQL Password"
echo "   - Claude API Key"
echo ""
read -p "シークレットの設定は完了していますか？ (y/N): " confirm
if [[ ! $confirm =~ ^[Yy]$ ]]; then
    echo "❌ シークレットの設定を完了してからデプロイしてください"
    exit 1
fi

# デプロイの実行
echo "🎯 デプロイを実行中..."

if [ "$USE_KUSTOMIZE" = true ]; then
    echo "📦 Kustomize を使用してデプロイ..."
    kustomize build $KUSTOMIZE_DIR | kubectl apply -f -
else
    echo "📦 kubectl apply -k を使用してデプロイ..."
    kubectl apply -k $KUSTOMIZE_DIR
fi

# デプロイ状況の確認
echo "⏳ デプロイ状況を確認中..."

# 名前空間の確認
kubectl get namespace $NAMESPACE 2>/dev/null || {
    echo "❌ 名前空間 $NAMESPACE の作成に失敗しました"
    exit 1
}

# PostgreSQLの起動を待機
echo "🗄️  PostgreSQL の起動を待機中..."
kubectl wait --for=condition=ready pod -l app=postgresql -n $NAMESPACE --timeout=300s

if [ $? -eq 0 ]; then
    echo "✅ PostgreSQL が正常に起動しました"
else
    echo "❌ PostgreSQL の起動がタイムアウトしました"
    kubectl logs -l app=postgresql -n $NAMESPACE --tail=50
    exit 1
fi

# スキーマ初期化ジョブの完了を待機
echo "🔧 データベーススキーマの初期化を待機中..."
kubectl wait --for=condition=complete job/postgresql-schema-init -n $NAMESPACE --timeout=180s

if [ $? -eq 0 ]; then
    echo "✅ データベーススキーマの初期化が完了しました"
else
    echo "❌ データベーススキーマの初期化に失敗しました"
    kubectl logs job/postgresql-schema-init -n $NAMESPACE
    exit 1
fi

# Discord Botの起動を待機
echo "🤖 Discord Bot の起動を待機中..."
kubectl wait --for=condition=ready pod -l app=discord-claude-bot -n $NAMESPACE --timeout=300s

if [ $? -eq 0 ]; then
    echo "✅ Discord Bot が正常に起動しました"
else
    echo "❌ Discord Bot の起動がタイムアウトしました"
    kubectl logs -l app=discord-claude-bot -n $NAMESPACE --tail=50
    exit 1
fi

# 最終確認
echo ""
echo "🎉 デプロイが完了しました！"
echo ""
echo "📊 システム状態:"
kubectl get pods,svc,pvc -n $NAMESPACE

echo ""
echo "📝 次のステップ:"
echo "1. Discord サーバーで Bot をテスト"
echo "2. /claude start コマンドでサンドボックスをテスト"
echo "3. ログを確認: kubectl logs -l app=discord-claude-bot -n $NAMESPACE -f"
echo ""
echo "🔍 トラブルシューティング:"
echo "- ポッド状態確認: kubectl describe pod -l app=discord-claude-bot -n $NAMESPACE"
echo "- PostgreSQL確認: kubectl logs -l app=postgresql -n $NAMESPACE"
echo "- 設定確認: kubectl get configmap,secret -n $NAMESPACE"