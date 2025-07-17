#!/bin/bash

# Discord Claude システムのクリーンアップスクリプト
set -e

NAMESPACE="discord-claude"

echo "🧹 Discord Claude システムのクリーンアップを開始します..."

# 確認プロンプト
echo "⚠️  このスクリプトは以下を削除します:"
echo "   - 名前空間: $NAMESPACE"
echo "   - すべてのアプリケーションデータ"
echo "   - PostgreSQL データベース（PV は保持）"
echo "   - 実行中のサンドボックス"
echo ""
read -p "本当にクリーンアップを実行しますか？ (y/N): " confirm
if [[ ! $confirm =~ ^[Yy]$ ]]; then
    echo "❌ クリーンアップをキャンセルしました"
    exit 1
fi

# 実行中のサンドボックスの確認と削除
echo "🔧 実行中のサンドボックスを確認中..."
SANDBOXES=$(kubectl get pods -n $NAMESPACE -l app=claude-sandbox --no-headers 2>/dev/null || echo "")

if [ -n "$SANDBOXES" ]; then
    echo "📦 実行中のサンドボックスを削除中..."
    kubectl delete pods -n $NAMESPACE -l app=claude-sandbox --force --grace-period=0
    echo "✅ サンドボックスを削除しました"
else
    echo "📦 実行中のサンドボックスはありません"
fi

# アプリケーションの削除
echo "🤖 Discord Bot を削除中..."
kubectl delete deployment discord-claude-bot -n $NAMESPACE --ignore-not-found=true

echo "🗄️  PostgreSQL を削除中..."
kubectl delete deployment postgresql -n $NAMESPACE --ignore-not-found=true

# ジョブの削除
echo "⚙️  初期化ジョブを削除中..."
kubectl delete job postgresql-schema-init -n $NAMESPACE --ignore-not-found=true
kubectl delete job postgresql-init -n $NAMESPACE --ignore-not-found=true

# サービスの削除
echo "🌐 サービスを削除中..."
kubectl delete svc -n $NAMESPACE --all

# ConfigMap と Secret の削除
echo "🔐 設定とシークレットを削除中..."
kubectl delete configmap -n $NAMESPACE --all
kubectl delete secret -n $NAMESPACE --all

# PVC の削除（データが削除されます）
echo "💾 PVC を削除中..."
read -p "PVC（データベースデータ）も削除しますか？ (y/N): " confirm_pvc
if [[ $confirm_pvc =~ ^[Yy]$ ]]; then
    kubectl delete pvc -n $NAMESPACE --all
    echo "✅ PVC を削除しました"
else
    echo "⚠️  PVC は保持されます"
fi

# RBAC の削除
echo "🔒 RBAC を削除中..."
kubectl delete rolebinding -n $NAMESPACE --all
kubectl delete role -n $NAMESPACE --all
kubectl delete serviceaccount -n $NAMESPACE --all

# ClusterRole と ClusterRoleBinding の削除
kubectl delete clusterrolebinding discord-claude-namespace-manager --ignore-not-found=true
kubectl delete clusterrole discord-claude-namespace-manager --ignore-not-found=true
kubectl delete clusterrolebinding nfs-provisioner --ignore-not-found=true
kubectl delete clusterrole nfs-provisioner --ignore-not-found=true

# 名前空間の削除
echo "🏠 名前空間を削除中..."
kubectl delete namespace $NAMESPACE --ignore-not-found=true

# PV の削除（オプション）
echo "🗂️  PV の削除確認..."
read -p "PV（PostgreSQL用）も削除しますか？ (y/N): " confirm_pv
if [[ $confirm_pv =~ ^[Yy]$ ]]; then
    kubectl delete pv postgresql-pv --ignore-not-found=true
    echo "✅ PV を削除しました"
else
    echo "⚠️  PV は保持されます"
fi

# StorageClass の削除（オプション）
echo "📦 StorageClass の削除確認..."
read -p "StorageClass（nfs-storage）も削除しますか？ (y/N): " confirm_sc
if [[ $confirm_sc =~ ^[Yy]$ ]]; then
    kubectl delete storageclass nfs-storage --ignore-not-found=true
    echo "✅ StorageClass を削除しました"
else
    echo "⚠️  StorageClass は保持されます"
fi

# 最終確認
echo ""
echo "🎉 クリーンアップが完了しました！"
echo ""
echo "📊 残存リソース確認:"
echo "PV:"
kubectl get pv | grep discord-claude || echo "  なし"
echo "StorageClass:"
kubectl get storageclass nfs-storage 2>/dev/null || echo "  なし"
echo ""
echo "💡 再デプロイする場合は ./scripts/deploy.sh を実行してください"