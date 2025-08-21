package redfishstatus

import (
	"reflect"
	"strings"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"

	"go.uber.org/zap"
)

func formatRedfishStatusName(ip string) string {
	return strings.ReplaceAll(ip, ".", "-")
}

// 比较两个Status的内容是否相同，忽略指针等问题
func compareRedfishStatus(a, b *topohubv1beta1.RedfishStatusStatus, logger *zap.SugaredLogger) bool {
	// 处理空指针情况
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
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

	if !reflect.DeepEqual(a.Basic, b.Basic) {
		if logger != nil {
			logger.Debugf("compareRedfishStatus Basic changed: %v -> %v", b.Basic, a.Basic)
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
