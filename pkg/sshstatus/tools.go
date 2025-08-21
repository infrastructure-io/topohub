package sshstatus

import (
	"fmt"
	"reflect"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// compareSSHStatus checks if two SSHStatus are equal, ignoring pointer issues
func compareSSHStatus(a, b *topohubv1beta1.SSHStatusStatus, logger *zap.SugaredLogger) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
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

	if !reflect.DeepEqual(a.Basic, b.Basic) {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic changed: %v -> %v", b.Basic, a.Basic)
		}
		return false
	}

	// Compare Info fields
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

// GenerateEvents creates Kubernetes events from SSH log entries and returns the latest messages and counts
func (c *sshStatusController) GenerateEvents(logEntries []map[string]string, sshStatusName, lastLogTime string) (newLastestTime, newLastestMsg, newLastestWarningTime, newLastestWarningMsg string, totalMsgCount, warningMsgCount, newLogAccount int) {
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

		// All new logs generate events
		if timestamp != lastLogTime {
			newLogAccount++
			c.log.Infof("find new log for sshStatus %s: %s", sshStatusName, msg)

			// Find the latest log
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
