package hoststatus

import (
	"context"
	"fmt"
	"strings"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/infrastructure-io/topohub/pkg/log"
	"go.uber.org/zap"
)

// getSecretData 从 Secret 中获取用户名和密码
func (c *hostStatusController) getSecretData(secretName, secretNamespace string) (string, string, error) {
	log.Logger.Debugf("Attempting to get secret data for %s/%s", secretNamespace, secretName)

	log.Logger.Debugf("Fetching secret from Kubernetes API for %s/%s", secretNamespace, secretName)
	// 如果不同，从 Secret 中获取认证信息
	secret, err := c.kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("Failed to get secret %s/%s: %v", secretNamespace, secretName, err)
		return "", "", err
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	log.Logger.Debugf("Successfully retrieved secret data for %s/%s", secretNamespace, secretName)
	return username, password, nil
}

func formatHostStatusName(ip string) string {
	return fmt.Sprintf("%s", strings.ReplaceAll(ip, ".", "-"))
}

// 比较两个Status的内容是否相同，忽略指针等问题
func compareHostStatus(a, b topohubv1beta1.HostStatusStatus, logger *zap.SugaredLogger) bool {
	if a.Healthy != b.Healthy {
		if logger != nil {
			logger.Debugf("compareHostStatus Healthy changed: %v -> %v", b.Healthy, a.Healthy)
		}
		return false
	}
	if a.ClusterName != b.ClusterName {
		if logger != nil {
			logger.Debugf("compareHostStatus ClusterName changed: %v -> %v", b.ClusterName, a.ClusterName)
		}
		return false
	}
	if a.LastUpdateTime != b.LastUpdateTime {
		if logger != nil {
			logger.Debugf("compareHostStatus LastUpdateTime changed: %v -> %v", b.LastUpdateTime, a.LastUpdateTime)
		}
		return false
	}

	// 比较Basic字段
	if a.Basic.Type != b.Basic.Type {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.Type changed: %v -> %v", b.Basic.Type, a.Basic.Type)
		}
		return false
	}
	if a.Basic.IpAddr != b.Basic.IpAddr {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.IpAddr changed: %v -> %v", b.Basic.IpAddr, a.Basic.IpAddr)
		}
		return false
	}
	if a.Basic.SecretName != b.Basic.SecretName {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.SecretName changed: %v -> %v", b.Basic.SecretName, a.Basic.SecretName)
		}
		return false
	}
	if a.Basic.SecretNamespace != b.Basic.SecretNamespace {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.SecretNamespace changed: %v -> %v", b.Basic.SecretNamespace, a.Basic.SecretNamespace)
		}
		return false
	}
	if a.Basic.Https != b.Basic.Https {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.Https changed: %v -> %v", b.Basic.Https, a.Basic.Https)
		}
		return false
	}
	if a.Basic.Port != b.Basic.Port {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.Port changed: %v -> %v", b.Basic.Port, a.Basic.Port)
		}
		return false
	}
	if a.Basic.Mac != b.Basic.Mac {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.Mac changed: %v -> %v", b.Basic.Mac, a.Basic.Mac)
		}
		return false
	}
	if a.Basic.ActiveDhcpClient != b.Basic.ActiveDhcpClient {
		if logger != nil {
			logger.Debugf("compareHostStatus Basic.ActiveDhcpClient changed: %v -> %v", b.Basic.ActiveDhcpClient, a.Basic.ActiveDhcpClient)
		}
		return false
	}

	// 比较Info map中的内容
	if len(a.Info) != len(b.Info) {
		if logger != nil {
			logger.Debugf("compareHostStatus Info length changed: %d -> %d", len(b.Info), len(a.Info))
		}
		return false
	}
	for k, v1 := range a.Info {
		if v2, ok := b.Info[k]; !ok || v1 != v2 {
			if logger != nil {
				if !ok {
					logger.Debugf("compareHostStatus Info added new key: %s = %s", k, v1)
				} else {
					logger.Debugf("compareHostStatus Info key %s changed: %s -> %s", k, v2, v1)
				}
			}
			return false
		}
	}
	// 检查是否有删除的键
	for k := range b.Info {
		if _, ok := a.Info[k]; !ok {
			if logger != nil {
				logger.Debugf("compareHostStatus Info removed key: %s", k)
			}
			return false
		}
	}
	return true
}
