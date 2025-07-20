package k8s

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hirano00o/disclaude/internal/config"
	"github.com/hirano00o/disclaude/internal/db"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
)

// SandboxManager はサンドボックスの管理を行う
type SandboxManager struct {
	client *Client
	db     *db.DB
	config *config.Config
}

// NewSandboxManager は新しいSandboxManagerを作成する
func NewSandboxManager(client *Client, database *db.DB, cfg *config.Config) *SandboxManager {
	return &SandboxManager{
		client: client,
		db:     database,
		config: cfg,
	}
}

// CreateSandbox はサンドボックス（Pod）を作成する
func (s *SandboxManager) CreateSandbox(ctx context.Context, sessionID int, threadID string) (*db.Sandbox, error) {
	// サンドボックス使用状況の確認
	usage, err := s.db.GetSandboxUsage()
	if err != nil {
		return nil, fmt.Errorf("failed to get sandbox usage: %w", err)
	}

	if !usage.CanCreateSandbox() {
		return nil, fmt.Errorf("サンドボックスの上限に達しています（%d/%d）", usage.CurrentCount, usage.MaxCount)
	}

	// Pod名の生成
	podName := fmt.Sprintf("claude-sandbox-%s", strings.ReplaceAll(threadID, "_", "-"))

	// データベースにサンドボックス情報を記録
	sandbox, err := s.db.CreateSandbox(sessionID, podName, s.client.namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox record: %w", err)
	}

	// Pod仕様の作成
	pod := s.createPodSpec(podName, threadID)

	// Podの作成
	podClient := s.client.clientset.CoreV1().Pods(s.client.namespace)
	createdPod, err := podClient.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		// 失敗時はデータベースからも削除
		s.db.UpdateSandboxStatus(sandbox.ID, "failed")
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	// サンドボックス使用数を増加
	if err := s.db.IncrementSandboxUsage(); err != nil {
		logrus.WithError(err).Error("Failed to increment sandbox usage")
	}

	// サンドボックスステータスを更新
	if err := s.db.UpdateSandboxStatus(sandbox.ID, "running"); err != nil {
		logrus.WithError(err).Error("Failed to update sandbox status")
	}

	logrus.WithFields(logrus.Fields{
		"pod_name":   createdPod.Name,
		"namespace":  createdPod.Namespace,
		"session_id": sessionID,
		"thread_id":  threadID,
	}).Info("Sandbox created successfully")

	return sandbox, nil
}

// createPodSpec はPod仕様を作成する
func (s *SandboxManager) createPodSpec(podName, threadID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: s.client.namespace,
			Labels: map[string]string{
				"app":       "claude-sandbox",
				"thread-id": threadID,
				"component": "disclaude",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "claude-code",
					Image: "anthropic/claude-code:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					Env: []corev1.EnvVar{
						{
							Name: "ANTHROPIC_API_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "claude-secrets",
									},
									Key: "api-key",
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "claude-config",
							MountPath: s.config.Claude.ConfigPath,
						},
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
					},
					WorkingDir: "/workspace",
					Command:    []string{"/bin/sh"},
					Args:       []string{"-c", "while true; do sleep 30; done"},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "claude-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "claude-config",
							},
						},
					},
				},
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

// DeleteSandbox はサンドボックス（Pod）を削除する
func (s *SandboxManager) DeleteSandbox(ctx context.Context, podName string) error {
	podClient := s.client.clientset.CoreV1().Pods(s.client.namespace)

	// Podの削除
	err := podClient.Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	// データベースの更新
	sandbox, err := s.db.GetSandboxByPodName(podName)
	if err != nil {
		logrus.WithError(err).Error("Failed to get sandbox for cleanup")
	} else if sandbox != nil {
		if err := s.db.UpdateSandboxStatus(sandbox.ID, "terminated"); err != nil {
			logrus.WithError(err).Error("Failed to update sandbox status")
		}
	}

	// サンドボックス使用数を減少
	if err := s.db.DecrementSandboxUsage(); err != nil {
		logrus.WithError(err).Error("Failed to decrement sandbox usage")
	}

	logrus.WithFields(logrus.Fields{
		"pod_name":  podName,
		"namespace": s.client.namespace,
	}).Info("Sandbox deleted successfully")

	return nil
}

// ExecuteCommand はサンドボックス内でコマンドを実行する
func (s *SandboxManager) ExecuteCommand(ctx context.Context, podName, command string) (string, error) {
	podClient := s.client.clientset.CoreV1().Pods(s.client.namespace)

	// Podの存在確認
	pod, err := podClient.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return "", fmt.Errorf("pod is not running: %s", pod.Status.Phase)
	}

	// コマンド実行の準備
	req := s.client.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(s.client.namespace).
		SubResource("exec").
		Param("container", "claude-code").
		Param("command", "/bin/sh").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	// リモートコマンド実行の設定
	exec, err := remotecommand.NewSPDYExecutor(s.client.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	// 入出力の準備
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader(command + "\nexit\n")

	// コマンド実行
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	// 結果の結合
	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\nSTDERR:\n" + stderr.String()
	}

	return result, nil
}

// GetSandboxStatus はサンドボックスのステータスを取得する
func (s *SandboxManager) GetSandboxStatus(ctx context.Context, podName string) (string, error) {
	podClient := s.client.clientset.CoreV1().Pods(s.client.namespace)

	pod, err := podClient.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod: %w", err)
	}

	return string(pod.Status.Phase), nil
}

// WaitForSandboxReady はサンドボックスが準備完了になるまで待機する
func (s *SandboxManager) WaitForSandboxReady(ctx context.Context, podName string, timeout time.Duration) error {
	podClient := s.client.clientset.CoreV1().Pods(s.client.namespace)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for pod to be ready")
		default:
			pod, err := podClient.Get(timeoutCtx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod: %w", err)
			}

			if pod.Status.Phase == corev1.PodRunning {
				// すべてのコンテナが準備完了かチェック
				allReady := true
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if !containerStatus.Ready {
						allReady = false
						break
					}
				}

				if allReady {
					return nil
				}
			}

			if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
				return fmt.Errorf("pod failed to start: %s", pod.Status.Phase)
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// ListSandboxes はサンドボックス一覧を取得する
func (s *SandboxManager) ListSandboxes(ctx context.Context) ([]corev1.Pod, error) {
	podClient := s.client.clientset.CoreV1().Pods(s.client.namespace)

	listOptions := metav1.ListOptions{
		LabelSelector: "app=claude-sandbox",
	}

	podList, err := podClient.List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return podList.Items, nil
}
