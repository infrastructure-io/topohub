// Complete the redfish information update for redfishstatus

package redfishstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/redfish"
	redfishstatusdata "github.com/infrastructure-io/topohub/pkg/redfishstatus/data"
	gofishredfish "github.com/stmcginnis/gofish/redfish"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ------------------------------  update the spec.info of the redfishstatus
func (c *redfishStatusController) GenerateEvents(logEntrys []*gofishredfish.LogEntry, redfishStatusName string, lastLogTime string,
) (newLastestTime, newLastestMsg, newLastestWarningTime, newLastestWarningMsg string, totalMsgCount, warningMsgCount, newLogAccount int) {
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

		// Generate events for all new logs
		if entry.Created != lastLogTime {
			newLogAccount++
			c.log.Infof("find new log for redfishStatus %s: %s", redfishStatusName, msg)

			// Confirm if there are new logs
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

// UpdateRedfishStatusInfo updates redfishstatus spec.info
func (c *redfishStatusController) UpdateRedfishStatusInfo(name string, d *redfishstatusdata.RedfishConnectCon) (bool, error) {
	// lock for updateing redfishStatus instance
	c.log.Debugf("lock for updateing redfishStatus instance %s", name)
	lock := lock.LockManagerInstance.GetLock(name)
	lock.Lock()
	defer lock.Unlock()

	// Create redfish client
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

	hasAuth := len(d.Username) > 0 && len(d.Password) > 0
	c.log.Debugf("try to check redfish with url: %s://%s:%d (auth: %v)", protocol, d.Info.IpAddr, d.Info.Port, hasAuth)

	// Get existing RedfishStatus
	existing := &topohubv1beta1.RedfishStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err != nil {
		c.log.Errorf("Failed to get RedfishStatus %s: %v", name, err)
		return false, err
	}
	updated := existing.DeepCopy()

	// Update health status
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

	// Update log
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

	// Update RedfishStatus
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

// UpdateRedfishStatusInfoWrapper updates redfishstatus spec.info
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

// UpdateRedfishStatusAtInterval updates redfishstatus spec.info at interval
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

// updateRedfishStatusResource updates RedfishStatus resource status
func (c *redfishStatusController) updateRedfishStatusResource(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {
	if err := c.client.Status().Update(context.TODO(), redfishStatus); err != nil {
		logger.Errorf("Failed to update status of RedfishStatus %s: %v", redfishStatus.Name, err)
		return err
	}
	logger.Infof("Successfully updated status for RedfishStatus %s", redfishStatus.Name)
	return nil
}

// processRedfishStatus reconciles redfishStatus, trigger updates
func (c *redfishStatusController) processRedfishStatus(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {
	// Step 1: Process RedfishStatus based on mode (DHCP or HostEndpoint)
	if err := c.processRedfishStatusByMode(redfishStatus, logger); err != nil {
		return err
	}

	// Step 2: Cache the redfishStatus data locally
	if err := c.cacheRedfishStatusData(redfishStatus, logger); err != nil {
		return err
	}

	// Step 3: Update RedfishStatus info if needed
	return c.updateRedfishStatusInfoIfNeeded(redfishStatus, logger)
}

// processRedfishStatusByMode handles the mode-specific processing logic
func (c *redfishStatusController) processRedfishStatusByMode(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {
	// DHCP client mode: load basic info from annotation and verify connection
	if redfishStatus.ObjectMeta.Labels != nil && redfishStatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] == topohubv1beta1.HostTypeDHCP {
		return c.processDHCPMode(redfishStatus, logger)
	}

	// HostEndpoint mode: get HostEndpoint info from OwnerReference
	if len(redfishStatus.Status.Basic.IpAddr) == 0 && len(redfishStatus.OwnerReferences) > 0 {
		return c.processHostEndpointMode(redfishStatus, logger)
	}

	return nil
}

// processDHCPMode handles DHCP client mode processing
func (c *redfishStatusController) processDHCPMode(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {
	if redfishStatus.ObjectMeta.Annotations == nil {
		logger.Warnf("RedfishStatus %s has no annotations", redfishStatus.Name)
		return nil
	}

	basicInfoJSON, ok := redfishStatus.ObjectMeta.Annotations[BasicInfoAnnotation]
	if !ok || basicInfoJSON == "" {
		return nil
	}

	// Parse basic info from annotation
	var basicInfo topohubv1beta1.BasicInfo
	if err := json.Unmarshal([]byte(basicInfoJSON), &basicInfo); err != nil {
		logger.Errorf("Failed to unmarshal basicInfo from annotation for RedfishStatus %s: %v", redfishStatus.Name, err)
		return nil
	}

	// Update RedfishStatus with basic info
	c.updateRedfishStatusWithBasicInfo(redfishStatus, &basicInfo)
	logger.Infof("Successfully loaded basicInfo from annotation for RedfishStatus %s", redfishStatus.Name)

	// Update status to Kubernetes
	if err := c.updateRedfishStatusResource(redfishStatus, logger); err != nil {
		return err
	}

	// Verify Redfish connection for DHCP clients
	return c.verifyRedfishConnectionForDHCP(redfishStatus, &basicInfo, logger)
}

// processHostEndpointMode handles HostEndpoint mode processing
func (c *redfishStatusController) processHostEndpointMode(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {
	for _, ownerRef := range redfishStatus.OwnerReferences {
		if ownerRef.Kind == topohubv1beta1.KindHostEndpoint {
			logger.Infof("Found HostEndpoint owner reference: %s", ownerRef.Name)

			// Get HostEndpoint
			hostEndpoint := &topohubv1beta1.HostEndpoint{}
			if err := c.client.Get(context.TODO(), client.ObjectKey{Name: ownerRef.Name}, hostEndpoint); err != nil {
				logger.Errorf("Failed to get HostEndpoint %s: %v", ownerRef.Name, err)
				return err
			}

			// Update RedfishStatus with HostEndpoint information
			c.updateRedfishStatusWithHostEndpoint(redfishStatus, hostEndpoint)

			// Update status to Kubernetes
			if err := c.updateRedfishStatusResource(redfishStatus, logger); err != nil {
				return err
			}
			logger.Infof("Successfully updated RedfishStatus %s with basic information", redfishStatus.Name)
			break
		}
	}
	return nil
}

// updateRedfishStatusWithBasicInfo updates RedfishStatus with basic info from annotation
func (c *redfishStatusController) updateRedfishStatusWithBasicInfo(redfishStatus *topohubv1beta1.RedfishStatus, basicInfo *topohubv1beta1.BasicInfo) {
	redfishStatus.Status.Basic = *basicInfo
	if redfishStatus.Status.Info == nil {
		redfishStatus.Status.Info = make(map[string]string)
	}
	redfishStatus.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
}

// updateRedfishStatusWithHostEndpoint updates RedfishStatus with HostEndpoint information
func (c *redfishStatusController) updateRedfishStatusWithHostEndpoint(redfishStatus *topohubv1beta1.RedfishStatus, hostEndpoint *topohubv1beta1.HostEndpoint) {
	clusterName := ""
	if hostEndpoint.Spec.ClusterName != nil {
		clusterName = *hostEndpoint.Spec.ClusterName
	}

	redfishStatus.Status = topohubv1beta1.RedfishStatusStatus{
		Healthy:        false,
		LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
		Basic: topohubv1beta1.BasicInfo{
			Type:        topohubv1beta1.HostTypeEndpoint,
			IpAddr:      hostEndpoint.Spec.IPAddr,
			Https:       true,
			Port:        443,
			ClusterName: clusterName,
		},
		Info: map[string]string{},
		Log: topohubv1beta1.LogStruct{
			TotalLogAccount:   0,
			WarningLogAccount: 0,
			LastestLog:        nil,
			LastestWarningLog: nil,
		},
	}

	// Set optional fields
	if hostEndpoint.Spec.SecretName != nil {
		redfishStatus.Status.Basic.SecretName = *hostEndpoint.Spec.SecretName
	}
	if hostEndpoint.Spec.SecretNamespace != nil {
		redfishStatus.Status.Basic.SecretNamespace = *hostEndpoint.Spec.SecretNamespace
	}
	if hostEndpoint.Spec.HTTPS != nil {
		redfishStatus.Status.Basic.Https = *hostEndpoint.Spec.HTTPS
	}
	if hostEndpoint.Spec.Port != nil {
		redfishStatus.Status.Basic.Port = *hostEndpoint.Spec.Port
	}
}

// verifyRedfishConnectionForDHCP verifies Redfish connection for DHCP clients
func (c *redfishStatusController) verifyRedfishConnectionForDHCP(redfishStatus *topohubv1beta1.RedfishStatus, basicInfo *topohubv1beta1.BasicInfo, logger *zap.SugaredLogger) error {
	username, password, err := c.getSecretData(c.config.RedfishSecretName, c.config.RedfishSecretNamespace)
	if err != nil {
		logger.Errorf("Failed to get secret data from secret %s/%s when processing redfishstatus for %s: %v",
			c.config.RedfishSecretNamespace, c.config.RedfishSecretName, redfishStatus.Name, err)
		return err
	}

	d := redfishstatusdata.RedfishConnectCon{
		Info:     basicInfo,
		Username: username,
		Password: password,
		DhcpHost: true,
	}

	if _, err := redfish.NewClient(d, logger); err != nil {
		logger.Warnf("Failed to connect to Redfish for DHCP client %s: %v", redfishStatus.Name, err)
		// We don't return error here, just log a warning and continue processing
	}
	return nil
}

// cacheRedfishStatusData caches the redfishStatus data locally
func (c *redfishStatusController) cacheRedfishStatusData(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {
	logger.Debugf("Processing RedfishStatus: %s (Type: %s, IP: %s, Health: %v)",
		redfishStatus.Name,
		redfishStatus.Status.Basic.Type,
		redfishStatus.Status.Basic.IpAddr,
		redfishStatus.Status.Healthy)

	// check if the redfishStatus data is already cached
	existingData := redfishstatusdata.RedfishCacheDatabase.Get(redfishStatus.Name)

	// create a new connection object
	var newConnectCon redfishstatusdata.RedfishConnectCon
	if existingData != nil {
		newConnectCon = redfishstatusdata.RedfishConnectCon{
			Info:     &redfishStatus.Status.Basic,
			Username: existingData.Username,
			Password: existingData.Password,
			DhcpHost: redfishStatus.Status.Basic.Type == topohubv1beta1.HostTypeDHCP,
		}
		if reflect.DeepEqual(existingData, &newConnectCon) {
			logger.Debugf("RedfishStatus %s cache data unchanged, skipping update", redfishStatus.Name)
			return nil
		}
	} else {
		if len(redfishStatus.Status.Basic.SecretName) == 0 || len(redfishStatus.Status.Basic.SecretNamespace) == 0 {
			logger.Warnf("RedfishStatus %s has no secret name or namespace", redfishStatus.Name)
			return nil
		}

		// get the authentication information
		username, password, err := c.getSecretData(
			redfishStatus.Status.Basic.SecretName,
			redfishStatus.Status.Basic.SecretNamespace,
		)
		if err != nil {
			logger.Errorf("Failed to get secret data for RedfishStatus %s: %v", redfishStatus.Name, err)
			return err
		}
		newConnectCon = redfishstatusdata.RedfishConnectCon{
			Info:     &redfishStatus.Status.Basic,
			Username: username,
			Password: password,
			DhcpHost: redfishStatus.Status.Basic.Type == topohubv1beta1.HostTypeDHCP,
		}

	}

	// update the cache
	redfishstatusdata.RedfishCacheDatabase.Add(redfishStatus.Name, newConnectCon)
	logger.Debugf("Successfully cached RedfishStatus %s data (IP: %s)", redfishStatus.Name, redfishStatus.Status.Basic.IpAddr)
	return nil
}

// updateRedfishStatusInfoIfNeeded updates RedfishStatus info if needed
func (c *redfishStatusController) updateRedfishStatusInfoIfNeeded(redfishStatus *topohubv1beta1.RedfishStatus, logger *zap.SugaredLogger) error {
	if len(redfishStatus.Status.Info) == 0 {
		if err := c.UpdateRedfishStatusInfoWrapper(redfishStatus.Name); err != nil {
			//logger.Errorf("failed to update redfishStatus %s: %v", redfishStatus.Name, err)
			return err
		}
	} else {
		logger.Debugf("RedfishStatus %s has already been processed, skipping the first time update", redfishStatus.Name)
	}
	return nil
}

// Only leader executes Reconcile
// Reconcile implements the reconcile.Reconciler interface
// Responsible for the first update of redfish information after redfishstatus creation (subsequent updates are handled by UpdateRedfishStatusAtInterval)
func (c *redfishStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.With("redfishstatus", req.Name)

	logger.Debugf("Reconciling RedfishStatus %s", req.Name)

	// Get RedfishStatus
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

	// if IP address is not empty, cache the redfishStatus data and return
	if len(redfishStatus.Status.Basic.IpAddr) != 0 {
		if err := c.cacheRedfishStatusData(redfishStatus, logger); err != nil {
			logger.Error(err, "Failed to cache RedfishStatus data")
		}
		logger.Debugf("RedfishStatus %s has IP address, skipping further processing", redfishStatus.Name)
		return ctrl.Result{}, nil
	}

	// Process RedfishStatus (including getting basic information from OwnerReferences and updating status)
	if err := c.processRedfishStatus(redfishStatus, logger); err != nil {
		logger.Error(err, "Failed to process RedfishStatus, will retry")
		return ctrl.Result{
			RequeueAfter: time.Second * 2,
		}, nil
	}

	logger.Debugf("Successfully processed RedfishStatus %s", redfishStatus.Name)
	return ctrl.Result{}, nil
}
