package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client はKubernetesクライアントを管理する構造体
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	namespace string
}

// NewClient は新しいKubernetesクライアントを作成する
func NewClient(namespace string) (*Client, error) {
	config, err := getKubernetesConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
		namespace: namespace,
	}, nil
}

// getKubernetesConfig はKubernetesの設定を取得する
func getKubernetesConfig() (*rest.Config, error) {
	// まずクラスター内設定を試す
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// クラスター外の場合はkubeconfigを使用
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// 環境変数からkubeconfigパスを取得
	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		kubeconfig = kubeconfigEnv
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from flags: %w", err)
	}

	return config, nil
}

// GetClientset はKubernetesクライアントセットを返す
func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.clientset
}

// GetConfig はKubernetes設定を返す
func (c *Client) GetConfig() *rest.Config {
	return c.config
}

// GetNamespace は名前空間を返す
func (c *Client) GetNamespace() string {
	return c.namespace
}

// CreateNamespace は名前空間を作成する
func (c *Client) CreateNamespace(ctx context.Context) error {
	nsClient := c.clientset.CoreV1().Namespaces()

	// 名前空間の存在確認
	_, err := nsClient.Get(ctx, c.namespace, metav1.GetOptions{})
	if err == nil {
		// 既に存在する場合はスキップ
		return nil
	}

	// 名前空間の作成
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.namespace,
			Labels: map[string]string{
				"app": "disclaude",
			},
		},
	}

	_, err = nsClient.Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", c.namespace, err)
	}

	return nil
}

// DeleteNamespace は名前空間を削除する
func (c *Client) DeleteNamespace(ctx context.Context) error {
	nsClient := c.clientset.CoreV1().Namespaces()

	err := nsClient.Delete(ctx, c.namespace, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", c.namespace, err)
	}

	return nil
}

// IsNamespaceReady は名前空間が準備完了かチェックする
func (c *Client) IsNamespaceReady(ctx context.Context) (bool, error) {
	nsClient := c.clientset.CoreV1().Namespaces()

	namespace, err := nsClient.Get(ctx, c.namespace, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get namespace %s: %w", c.namespace, err)
	}

	return namespace.Status.Phase == corev1.NamespaceActive, nil
}
