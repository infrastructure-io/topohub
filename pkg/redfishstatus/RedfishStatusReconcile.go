// 完成对 redfishstatus 的 redfish 信息更新

package redfishstatus

import (
	"context"
	"fmt"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	redfishstatusdata "github.com/infrastructure-io/topohub/pkg/redfishstatus/data"

	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/redfish"

	gofishredfish "github.com/stmcginnis/gofish/redfish"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ------------------------------  update the spec.info of the redfishstatus

// GenerateEvents creates Kubernetes events from Redfish log entries and returns the latest message and count
func (c *redfishStatusController) GenerateEvents(logEntrys []*gofishredfish.LogEntry, redfishStatusName string, lastLogTime string) (newLastestTime, newLastestMsg, newLastestWarningTime, newLastestWarningMsg string, totalMsgCount, warningMsgCount, newLogAccount int) {
	totalMsgCount = 0
	warningMsgCount = 0
	newLogAccount = 0
	newLastestTime = ""
	newLastestMsg = ""
	newLastestWarningTime = ""
	newLastestWarningMsg = ""

	if len(logEntrys) == 0 {
		return
	}

	totalMsgCount = len(logEntrys)
	for m, entry := range logEntrys {
		//log.Logger.Debugf("log service entries[%d] timestamp: %+v", m, entry.Created)
		//log.Logger.Debugf("log service entries[%d] severity: %+v", m, entry.Severity)
		//log.Logger.Debugf("log service entries[%d] oemSensorType: %+v", m, entry.OemSensorType)
		//log.Logger.Debugf("log service entries[%d] message: %+v", m, entry.Message)

		msg := fmt.Sprintf("[%s][%s]: %s %s", entry.Created, entry.Severity, entry.OemSensorType, entry.Message)

		ty := corev1.EventTypeNormal
		if entry.Severity != gofishredfish.OKEventSeverity && entry.Severity != "" {
			ty = corev1.EventTypeWarning
			if newLastestWarningMsg == "" {
				newLastestWarningTime = entry.Created
				newLastestWarningMsg = msg
			}
			warningMsgCount++
		}

		// 所有的新日志，生成 event
		if entry.Created != lastLogTime {
			newLogAccount++
			c.log.Infof("find new log for redfishStatus %s: %s", redfishStatusName, msg)

			// 确认是否有新日志了
			if m == 0 {
				newLastestTime = entry.Created
				newLastestMsg = msg
			}

			// Create event
			t := &corev1.ObjectReference{
				Kind:       topohubv1beta1.KindredfishStatus,
				Name:       redfishStatusName,
				Namespace:  c.config.PodNamespace,
				APIVersion: topohubv1beta1.APIVersion,
			}
			c.recorder.Event(t, ty, "BMCLogEntry", msg)

		}
	}
	return
}

// this is called by UpdateRedfishStatusAtInterval and UpdateRedfishStatusInfoWrapper
func (c *redfishStatusController) UpdateRedfishStatusInfo(name string, d *redfishstatusdata.RedfishConnectCon) (bool, error) {
	// lock for updateing redfishStatus instance
	c.log.Debugf("lock for updateing redfishStatus instance %s", name)
	lock := lock.LockManagerInstance.GetLock(name)
	lock.Lock()
	defer lock.Unlock()

	// 创建 redfish 客户端
	var healthy bool
	client, err1 := redfish.NewClient(*d, c.log)
	if err1 != nil {
		c.log.Warnf("Failed to create redfish client for RedfishStatus %s: %v", name, err1)
		healthy = false
	} else {
		healthy = true
	}

	protocol := "http"
	if d.Info.Https {
		protocol = "https"
	}
	auth := "without username and password"
	if len(d.Username) != 0 && len(d.Password) != 0 {
		auth = "with username and password"
	}
	c.log.Debugf("try to check redfish with url: %s://%s:%d , %s", protocol, d.Info.IpAddr, d.Info.Port, auth)

	// 获取现有的 RedfishStatus
	existing := &topohubv1beta1.RedfishStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err != nil {
		c.log.Errorf("Failed to get RedfishStatus %s: %v", name, err)
		return false, err
	}
	updated := existing.DeepCopy()

	// 检查健康状态
	updated.Status.Healthy = healthy
	if healthy {
		infoData, err := client.GetInfo()
		if err != nil {
			c.log.Errorf("Failed to get info of RedfishStatus %s: %v", name, err)
			healthy = false
		} else {
			updated.Status.Info = infoData
		}
	}
	if !healthy {
		c.log.Debugf("RedfishStatus %s is not healthy, set info to empty", name)
		updated.Status.Info = map[string]string{}
	}
	if updated.Status.Healthy != existing.Status.Healthy {
		c.log.Infof("RedfishStatus %s change from %v to %v , update status", name, existing.Status.Healthy, healthy)
	}

	// 获取日志
	if healthy {
		logEntrys, err := client.GetLog()
		if err != nil {
			c.log.Warnf("Failed to get logs of RedfishStatus %s: %v", name, err)
		} else {
			lastLogTime := ""
			if updated.Status.Log.LastestLog != nil {
				lastLogTime = updated.Status.Log.LastestLog.Time
			}
			newLastestTime, newLastestMsg, newLastestWarningTime, newLastestWarningMsg, totalMsgCount, warningMsgCount, newLogAccount := c.GenerateEvents(logEntrys, name, lastLogTime)
			if newLastestTime != "" {
				updated.Status.Log.TotalLogAccount = int32(totalMsgCount)
				updated.Status.Log.WarningLogAccount = int32(warningMsgCount)
				updated.Status.Log.LastestLog = &topohubv1beta1.LogEntry{
					Time:    newLastestTime,
					Message: newLastestMsg,
				}
				updated.Status.Log.LastestWarningLog = &topohubv1beta1.LogEntry{
					Time:    newLastestWarningTime,
					Message: newLastestWarningMsg,
				}
				c.log.Infof("find %d new logs for redfishStatus %s", newLogAccount, name)
			}
		}
	}

	// 更新 RedfishStatus
	if !compareRedfishStatus(updated.Status, existing.Status, c.log) {
		c.log.Debugf("status changed, existing: %v, updated: %v", existing.Status, updated.Status)
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		if err := c.client.Status().Update(context.Background(), updated); err != nil {
			return true, err
		}
		c.log.Infof("Successfully updated redfishStatus %s status", name)
		return true, nil
	}
	return false, nil
}

// this is called by UpdateRedfishStatusAtInterval and
func (c *redfishStatusController) UpdateRedfishStatusInfoWrapper(name string) error {
	syncData := make(map[string]redfishstatusdata.RedfishConnectCon)
	modeinfo := ""
	if len(name) == 0 {
		syncData = redfishstatusdata.RedfishCacheDatabase.GetAll()
		if len(syncData) == 0 {
			return nil
		}
		modeinfo = " during periodic update"
	} else {
		d := redfishstatusdata.RedfishCacheDatabase.Get(name)
		if d != nil {
			syncData[name] = *d
		}
		if len(syncData) == 0 {
			c.log.Errorf("no cache data found for redfishStatus %s ", name)
			return fmt.Errorf("no cache data found for redfishStatus %s ", name)
		}
		modeinfo = " during redfishStatus reconcile"
	}

	failed := false
	for item, t := range syncData {
		c.log.Debugf("updating status of the redfishStatus %s", item)
		if updated, err := c.UpdateRedfishStatusInfo(item, &t); err != nil {
			c.log.Errorf("failed to update status of redfishStatus %s, %s: %v", item, modeinfo, err)
			failed = true
		} else {
			if updated {
				c.log.Debugf("succeeded to update status of the redfishStatus %s, %s", item, modeinfo)
			} else {
				c.log.Debugf("no need to update status of the redfishStatus %s, %s", item, modeinfo)
			}
		}
	}

	if failed {
		return fmt.Errorf("failed to update redfishStatus")
	}
	return nil
}

// ------------------------------  redfishstatus spec.info 的	周期更新
func (c *redfishStatusController) UpdateRedfishStatusAtInterval() {
	interval := time.Duration(c.config.RedfishStatusUpdateInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	c.log.Infof("begin to update all redfishStatus at interval of %v seconds", c.config.RedfishStatusUpdateInterval)

	for {
		select {
		case <-c.stopCh:
			c.log.Info("Stopping UpdateRedfishStatusAtInterval")
			return
		case <-ticker.C:
			c.log.Debugf("update all redfishStatus at interval ")
			if err := c.UpdateRedfishStatusInfoWrapper(""); err != nil {
				c.log.Errorf("Failed to update redfish status: %v", err)
			}
		}
	}
}

// ------------------------------  redfishStatus 的 reconcile , 触发更新

// 缓存 redfishStatus 数据本地，并行更新 status.info 信息
func (c *redfishStatusController) processRedfishStatus(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {

	logger.Debugf("Processing Existed RedfishStatus: %s (Type: %s, IP: %s, Health: %v)",
		redfishStatus.Name,
		redfishStatus.Status.Basic.Type,
		redfishStatus.Status.Basic.IpAddr,
		redfishStatus.Status.Healthy)

	// cache the redfishStatus data to local
	username := ""
	password := ""
	var err error
	if len(redfishStatus.Status.Basic.SecretName) > 0 && len(redfishStatus.Status.Basic.SecretNamespace) > 0 {
		username, password, err = c.getSecretData(
			redfishStatus.Status.Basic.SecretName,
			redfishStatus.Status.Basic.SecretNamespace,
		)
		if err != nil {
			logger.Errorf("Failed to get secret data for RedfishStatus %s: %v", redfishStatus.Name, err)
			return err
		}
		logger.Debugf("Adding/Updating RedfishStatus %s in cache with username: %s",
			redfishStatus.Name, username)
	} else {
		logger.Debugf("Adding/Updating RedfishStatus %s in cache with empty username", redfishStatus.Name)
	}

	redfishstatusdata.RedfishCacheDatabase.Add(redfishStatus.Name, redfishstatusdata.RedfishConnectCon{
		Info:     &redfishStatus.Status.Basic,
		Username: username,
		Password: password,
		DhcpHost: redfishStatus.Status.Basic.Type == topohubv1beta1.HostTypeDHCP,
	})

	if len(redfishStatus.Status.Info) == 0 {
		if err := c.UpdateRedfishStatusInfoWrapper(redfishStatus.Name); err != nil {
			//logger.Errorf("failed to update redfishStatus %s: %v", redfishStatus.Name, err)
			return err
		}
	} else {
		logger.Debugf("RedfishStatus %s has already been processed, skipping the first time update", redfishStatus.Name)
	}

	logger.Debugf("Successfully processed RedfishStatus %s", redfishStatus.Name)
	return nil
}

// 只有 leader 才会执行 Reconcile
// Reconcile 实现 reconcile.Reconciler 接口
// 负责在 redfishstatus 创建后 redfish 信息的第一次更新（后续的更新由 UpdateRedfishStatusAtInterval 完成）
func (c *redfishStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.With("redfishstatus", req.Name)

	logger.Debugf("Reconciling RedfishStatus %s", req.Name)

	// 获取 RedfishStatus
	redfishStatus := &topohubv1beta1.RedfishStatus{}
	if err := c.client.Get(ctx, req.NamespacedName, redfishStatus); err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("RedfishStatus not found")
			data := redfishstatusdata.RedfishCacheDatabase.Get(req.Name)
			if data != nil {
				// try to delete the binding setting in dhcp server config
				logger.Infof("delete redfishStatus %s in cache, %+v", req.Name, *data)
				redfishstatusdata.RedfishCacheDatabase.Delete(req.Name)
			}
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get RedfishStatus")
		return ctrl.Result{}, err
	}

	if len(redfishStatus.Status.Basic.IpAddr) == 0 {
		// the redfishStatus is created firstly and then be updated with its status
		c.log.Debugf("ingore redfishStatus %s just created", redfishStatus.Name)
		return ctrl.Result{}, nil
	}

	// 处理 RedfishStatus
	if err := c.processRedfishStatus(redfishStatus, logger); err != nil {
		logger.Error(err, "Failed to process RedfishStatus, will retry")
		return ctrl.Result{
			RequeueAfter: time.Second * 2,
		}, nil
	}

	logger.Debugf("Successfully processed RedfishStatus %s", redfishStatus.Name)
	return ctrl.Result{}, nil
}
