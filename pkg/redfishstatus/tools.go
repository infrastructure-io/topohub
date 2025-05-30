package redfishstatus

import (
	"context"
	"strings"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.uber.org/zap"
)

// getSecretData 从 Secret 中获取用户名和密码
func (c *redfishStatusController) getSecretData(secretName, secretNamespace string) (string, string, error) {
	c.log.Debugf("Attempting to get secret data for %s/%s", secretNamespace, secretName)

	c.log.Debugf("Fetching secret from Kubernetes API for %s/%s", secretNamespace, secretName)
	// 如果不同，从 Secret 中获取认证信息
	secret, err := c.kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		c.log.Errorf("Failed to get secret %s/%s: %v", secretNamespace, secretName, err)
		return "", "", err
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	c.log.Debugf("Successfully retrieved secret data for %s/%s", secretNamespace, secretName)
	return username, password, nil
}

func formatRedfishStatusName(ip string) string {
	return strings.ReplaceAll(ip, ".", "-")
}

// 比较两个Status的内容是否相同，忽略指针等问题
func compareRedfishStatus(a, b topohubv1beta1.RedfishStatusStatus, logger *zap.SugaredLogger) bool {
	if a.Healthy != b.Healthy {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Healthy changed: %v -> %v", b.Healthy, a.Healthy)
		}
		return false
	}
	if a.LastUpdateTime != b.LastUpdateTime {
		if logger != nil {
			logger.Debugf("compareRedfishStatus LastUpdateTime changed: %v -> %v", b.LastUpdateTime, a.LastUpdateTime)
		}
		return false
	}

	// 比较Basic字段
	if a.Basic.Type != b.Basic.Type {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic.Type changed: %v -> %v", b.Basic.Type, a.Basic.Type)
		}
		return false
	}
	if a.Basic.IpAddr != b.Basic.IpAddr {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic.IpAddr changed: %v -> %v", b.Basic.IpAddr, a.Basic.IpAddr)
		}
		return false
	}
	if a.Basic.SecretName != b.Basic.SecretName {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic.SecretName changed: %v -> %v", b.Basic.SecretName, a.Basic.SecretName)
		}
		return false
	}
	if a.Basic.SecretNamespace != b.Basic.SecretNamespace {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic.SecretNamespace changed: %v -> %v", b.Basic.SecretNamespace, a.Basic.SecretNamespace)
		}
		return false
	}
	if a.Basic.Https != b.Basic.Https {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic.Https changed: %v -> %v", b.Basic.Https, a.Basic.Https)
		}
		return false
	}
	if a.Basic.Port != b.Basic.Port {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic.Port changed: %v -> %v", b.Basic.Port, a.Basic.Port)
		}
		return false
	}
	if a.Basic.ClusterName != b.Basic.ClusterName {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic.ClusterName changed: %v -> %v", b.Basic.ClusterName, a.Basic.ClusterName)
		}
		return false
	}

	// 比较Info map中的内容
	if len(a.Info) != len(b.Info) {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Info length changed: %d -> %d", len(b.Info), len(a.Info))
		}
		return false
	}
	for k, v1 := range a.Info {
		if v2, ok := b.Info[k]; !ok || v1 != v2 {
			if logger != nil {
				if !ok {
					logger.Debugf("compareRedfishStatus Info added new key: %s = %s", k, v1)
				} else {
					logger.Debugf("compareRedfishStatus Info key %s changed: %s -> %s", k, v2, v1)
				}
			}
			return false
		}
	}
	// 检查是否有删除的键
	for k := range b.Info {
		if _, ok := a.Info[k]; !ok {
			if logger != nil {
				logger.Debugf("compareRedfishStatus Info removed key: %s", k)
			}
			return false
		}
	}
	return true
}
