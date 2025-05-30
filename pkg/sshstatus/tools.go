package sshstatus

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// getSecretData 从 Secret 中获取用户名和密码
func (c *sshStatusController) getSecretData(secretName, secretNamespace string) (string, string, string, bool, error) {
	c.log.Debugf("Attempting to get secret data for %s/%s", secretNamespace, secretName)

	c.log.Debugf("Fetching secret from Kubernetes API for %s/%s", secretNamespace, secretName)
	// 从 Secret 中获取认证信息
	secret, err := c.kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		c.log.Errorf("Failed to get secret %s/%s: %v", secretNamespace, secretName, err)
		return "", "", "", false, err
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	sshKey := string(secret.Data["ssh-privatekey"])
	sshKeyAuth := sshKey != ""

	c.log.Debugf("Successfully retrieved secret data for %s/%s", secretNamespace, secretName)
	return username, password, sshKey, sshKeyAuth, nil
}

// 比较两个SSHStatus的内容是否相同，忽略指针等问题
func compareSSHStatus(a, b topohubv1beta1.SSHStatusStatus, logger *zap.SugaredLogger) bool {
	if a.Healthy != b.Healthy {
		if logger != nil {
			logger.Debugf("compareSSHStatus Healthy changed: %v -> %v", b.Healthy, a.Healthy)
		}
		return false
	}
	if a.LastUpdateTime != b.LastUpdateTime {
		if logger != nil {
			logger.Debugf("compareSSHStatus LastUpdateTime changed: %v -> %v", b.LastUpdateTime, a.LastUpdateTime)
		}
		return false
	}

	// 比较Basic字段
	if a.Basic.Type != b.Basic.Type {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.Type changed: %v -> %v", b.Basic.Type, a.Basic.Type)
		}
		return false
	}
	if a.Basic.IpAddr != b.Basic.IpAddr {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.IpAddr changed: %v -> %v", b.Basic.IpAddr, a.Basic.IpAddr)
		}
		return false
	}
	if a.Basic.Port != b.Basic.Port {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.Port changed: %v -> %v", b.Basic.Port, a.Basic.Port)
		}
		return false
	}
	if a.Basic.SecretName != b.Basic.SecretName {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.SecretName changed: %v -> %v", b.Basic.SecretName, a.Basic.SecretName)
		}
		return false
	}
	if a.Basic.SecretNamespace != b.Basic.SecretNamespace {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.SecretNamespace changed: %v -> %v", b.Basic.SecretNamespace, a.Basic.SecretNamespace)
		}
		return false
	}
	if a.Basic.Username != b.Basic.Username {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.Username changed: %v -> %v", b.Basic.Username, a.Basic.Username)
		}
		return false
	}
	if a.Basic.SSHKeyAuth != b.Basic.SSHKeyAuth {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.SSHKeyAuth changed: %v -> %v", b.Basic.SSHKeyAuth, a.Basic.SSHKeyAuth)
		}
		return false
	}
	if a.Basic.ClusterName != b.Basic.ClusterName {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.ClusterName changed: %v -> %v", b.Basic.ClusterName, a.Basic.ClusterName)
		}
		return false
	}

	// 比较Info字段
	if len(a.Info) != len(b.Info) {
		if logger != nil {
			logger.Debugf("compareSSHStatus Info length changed: %v -> %v", len(b.Info), len(a.Info))
		}
		return false
	}
	for k, v := range a.Info {
		if bv, ok := b.Info[k]; !ok || bv != v {
			if logger != nil {
				logger.Debugf("compareSSHStatus Info[%s] changed: %v -> %v", k, bv, v)
			}
			return false
		}
	}

	return true
}

// GenerateEvents 从SSH日志条目创建Kubernetes事件并返回最新消息和计数
func (c *sshStatusController) GenerateEvents(logEntries []map[string]string, sshStatusName string, lastLogTime string) (newLastestTime, newLastestMsg, newLastestWarningTime, newLastestWarningMsg string, totalMsgCount, warningMsgCount, newLogAccount int) {
	totalMsgCount = 0
	warningMsgCount = 0
	newLogAccount = 0
	newLastestTime = ""
	newLastestMsg = ""
	newLastestWarningTime = ""
	newLastestWarningMsg = ""

	if len(logEntries) == 0 {
		return
	}

	totalMsgCount = len(logEntries)
	for i, entry := range logEntries {
		timestamp := entry["timestamp"]
		level := entry["level"]
		message := entry["message"]

		msg := fmt.Sprintf("[%s][%s]: %s", timestamp, level, message)

		ty := corev1.EventTypeNormal
		if level == "ERROR" || level == "WARNING" {
			ty = corev1.EventTypeWarning
			if newLastestWarningMsg == "" {
				newLastestWarningTime = timestamp
				newLastestWarningMsg = msg
			}
			warningMsgCount++
		}

		// 所有的新日志，生成 event
		if timestamp != lastLogTime {
			newLogAccount++
			c.log.Infof("find new log for sshStatus %s: %s", sshStatusName, msg)

			// 确认是否有新日志了
			if i == 0 {
				newLastestTime = timestamp
				newLastestMsg = msg
			}

			// Create event
			t := &corev1.ObjectReference{
				Kind:       topohubv1beta1.KindSSHStatus,
				Name:       sshStatusName,
				Namespace:  c.config.PodNamespace,
				APIVersion: topohubv1beta1.APIVersion,
			}
			c.recorder.Event(t, ty, "SSHLogEntry", msg)
		}
	}
	return
}
