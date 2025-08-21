// Complete the redfish information update for redfishstatus

package redfishstatus

import (
	"context"
	"fmt"
	"sync"
	"time"

	gofishredfish "github.com/stmcginnis/gofish/redfish"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/redfish"
	redfishstatusdata "github.com/infrastructure-io/topohub/pkg/redfishstatus/data"
	"github.com/infrastructure-io/topohub/pkg/tools"
)

// ------------------------------  update the spec.info of the redfishstatus
func (c *redfishStatusController) GenerateEvents(logEntrys []*gofishredfish.LogEntry, redfishStatusName, lastLogTime string,
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
		// log.Logger.Debugf("log service entries[%d] timestamp: %+v", m, entry.Created)
		// log.Logger.Debugf("log service entries[%d] severity: %+v", m, entry.Severity)
		// log.Logger.Debugf("log service entries[%d] oemSensorType: %+v", m, entry.OemSensorType)
		// log.Logger.Debugf("log service entries[%d] message: %+v", m, entry.Message)

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
func (c *redfishStatusController) UpdateRedfishStatusInfo(oldRedfishStatus *topohubv1beta1.RedfishStatus) error {
	name := oldRedfishStatus.Name

	// lock for updateing redfishStatus instance
	c.log.Debugf("lock for updateing redfishStatus instance %s", name)
	lock := lock.LockManagerInstance.GetLock(name)
	lock.Lock()
	defer lock.Unlock()

	// create a copy of redfishStatus
	if oldRedfishStatus.Status == nil {
		oldRedfishStatus.Status = &topohubv1beta1.RedfishStatusStatus{}
	}
	updated := oldRedfishStatus.DeepCopy()

	// get hostEndpoint
	hostEndpoint, err := c.getHostEndpoint(oldRedfishStatus)
	if err != nil {
		return fmt.Errorf("Failed to get hostEndpoint for RedfishStatus %s: %v", oldRedfishStatus.Name, err)
	}

	// get connection data
	username, password, err := c.getSecretData(
		*hostEndpoint.Spec.SecretName,
		*hostEndpoint.Spec.SecretNamespace,
	)
	if err != nil {
		return fmt.Errorf("Failed to get secret data for HostEndpoint %s: %v", hostEndpoint.Name, err)
	}
	connInfo := &redfishstatusdata.RedfishConnectCon{
		Username: username,
		Password: password,
		IPAddr:   hostEndpoint.Spec.IPAddr,
		Port:     int(*hostEndpoint.Spec.Port),
		Http:     !*hostEndpoint.Spec.HTTPS,
	}

	// update subnetName from range
	subnetName, err := tools.GetSubnetNameByIP(connInfo.IPAddr, c.client, c.log)
	if err != nil {
		c.log.Errorf("Failed to update subnet name from range: %v", err)
	} else {
		c.log.Infof("Updated subnetName to %s for RedfishStatus %s (IP: %s)", subnetName, name, connInfo.IPAddr)
		updated.Status.Basic.SubnetName = subnetName
	}

	// update clusterName
	if hostEndpoint.Spec.ClusterName != nil {
		updated.Status.Basic.ClusterName = *hostEndpoint.Spec.ClusterName
	}

	// Create redfish client
	var healthy bool
	client, err1 := redfish.NewClient(*connInfo, c.log)
	if err1 != nil {
		c.log.Warnf("Failed to create redfish client for RedfishStatus %s: %v", name, err1)
		healthy = false
	} else {
		healthy = true
	}

	defer func() {
		if client != nil {
			client.Logout()
		}
	}()

	protocol := "http"
	if !connInfo.Http {
		protocol = "https"
	}

	hasAuth := len(connInfo.Username) > 0 && len(connInfo.Password) > 0
	c.log.Debugf("try to check redfish with url: %s://%s:%d (auth: %v)", protocol, connInfo.IPAddr, connInfo.Port, hasAuth)

	// Update health status
	updated.Status.Healthy = healthy
	if healthy {
		infoData, err := client.GetInfo()
		if err != nil {
			c.log.Errorf("Failed to get info of RedfishStatus %s: %v", name, err)
			healthy = false
		} else {
			if infoData != nil {
				updated.Status.Info = infoData
			} else {
				c.log.Warnf("GetInfo returned nil for RedfishStatus %s", name)
				updated.Status.Info = make(map[string]string)
			}
		}
	}
	if !healthy {
		c.log.Debugf("RedfishStatus %s is not healthy, set info to empty", name)
		updated.Status.Info = map[string]string{}
	}
	if updated.Status.Healthy != oldRedfishStatus.Status.Healthy {
		c.log.Infof("RedfishStatus %s change from %v to %v , update status", name, oldRedfishStatus.Status.Healthy, healthy)
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
	if !compareRedfishStatus(updated.Status, oldRedfishStatus.Status, c.log) {
		c.log.Debugf("status changed, existing: %v, updated: %v", oldRedfishStatus.Status, updated.Status)
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		if err := c.client.Status().Update(context.Background(), updated); err != nil {
			return err
		}
		c.log.Infof("Successfully updated redfishStatus %s status", name)
	}
	return nil
}

// UpdateRedfishStatusInfoWrapper updates redfishstatus spec.info
func (c *redfishStatusController) UpdateRedfishStatusInfoWrapper(status *topohubv1beta1.RedfishStatus) error {
	// get redfishStatus list
	var redfishStatusList topohubv1beta1.RedfishStatusList = topohubv1beta1.RedfishStatusList{}
	modeinfo := ""
	listOpts := []client.ListOption{}
	// if status is nil, list all redfishStatus
	if status == nil {
		if err := c.client.List(context.Background(), &redfishStatusList, listOpts...); err != nil {
			c.log.Errorf("Failed to list RedfishStatus: %v", err)
			return err
		}
	} else {
		redfishStatusList.Items = append(redfishStatusList.Items, *status)
	}

	// update each redfishStatus
	for _, redfishStatus := range redfishStatusList.Items {
		c.log.Debugf("Updating status of RedfishStatus %s", redfishStatus.Name)
		if err := c.UpdateRedfishStatusInfo(&redfishStatus); err != nil {
			c.log.Errorf("Failed to update status of RedfishStatus %s%s: %v",
				redfishStatus.Name, modeinfo, err)
		}
	}
	return nil
}

func (c *redfishStatusController) getHostEndpoint(redfishStatus *topohubv1beta1.RedfishStatus) (*topohubv1beta1.HostEndpoint, error) {
	// all RedfishStatus should have ownerReferences
	if len(redfishStatus.OwnerReferences) > 0 {
		for _, ownerRef := range redfishStatus.OwnerReferences {
			if ownerRef.Kind == topohubv1beta1.KindHostEndpoint {
				c.log.Infof("Found HostEndpoint owner reference: %s", ownerRef.Name)

				// get HostEndpoint
				hostEndpoint := &topohubv1beta1.HostEndpoint{}
				if err := c.client.Get(context.TODO(), client.ObjectKey{Name: ownerRef.Name}, hostEndpoint); err != nil {
					c.log.Errorf("Failed to get HostEndpoint %s: %v", ownerRef.Name, err)
					return nil, err
				}

				return hostEndpoint, nil
			}
		}
	}

	return nil, fmt.Errorf("Failed to get connection info for RedfishStatus %s", redfishStatus.Name)
}

// UpdateRedfishStatusAtInterval start two timer tasks, respectively for high and low frequency updates
func (c *redfishStatusController) UpdateRedfishStatusAtInterval() {
	// start high frequency update task (update basic status fields every minute)
	go c.updateHighFrequencyFields()

	// start low frequency update task (update detailed info and logs every day)
	go c.updateLowFrequencyFields()

	c.log.Infof("Started dual-frequency update tasks for RedfishStatus: high-freq=%vs, low-freq=%vs",
		c.config.RedfishStatusBasicUpdateInterval, c.config.RedfishStatusInfoUpdateInterval)
}

// updateHighFrequencyFields update RedfishStatus basic status fields (PowerState, BmcStatus and healthy) at interval
func (c *redfishStatusController) updateHighFrequencyFields() {
	interval := time.Duration(c.config.RedfishStatusBasicUpdateInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	c.log.Infof("Begin high-frequency updates (basic status fields) at interval of %v seconds", c.config.RedfishStatusBasicUpdateInterval)

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			c.log.Info("Stopping high-frequency updates")
			return
		case <-ticker.C:
			c.log.Debugf("Running high-frequency updates for all RedfishStatus")
			if err := c.updateBasicStatusForAll(); err != nil {
				c.log.Errorf("Failed to update basic status for all RedfishStatus: %v", err)
			}
		}
	}
}

// updateLowFrequencyFields update RedfishStatus detailed info and logs at interval
func (c *redfishStatusController) updateLowFrequencyFields() {
	interval := time.Duration(c.config.RedfishStatusInfoUpdateInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	c.log.Infof("Begin low-frequency updates (detailed info and logs) at interval of %v seconds", c.config.RedfishStatusInfoUpdateInterval)

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			c.log.Info("Stopping low-frequency updates")
			return
		case <-ticker.C:
			c.log.Debugf("Running low-frequency updates for all RedfishStatus")
			if err := c.UpdateRedfishStatusInfoWrapper(nil); err != nil {
				c.log.Errorf("Failed to update detailed redfish status: %v", err)
			}
		}
	}
}

// updateBasicStatusForAll update RedfishStatus basic status fields (PowerState, BmcStatus and healthy)
// using the shared ants goroutine pool
func (c *redfishStatusController) updateBasicStatusForAll() error {
	// get RedfishStatus list
	var redfishStatusList topohubv1beta1.RedfishStatusList
	listOpts := []client.ListOption{}
	if err := c.client.List(context.Background(), &redfishStatusList, listOpts...); err != nil {
		c.log.Errorf("Failed to list RedfishStatus for basic status update: %v", err)
		return err
	}

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Use the shared ants pool
	if c.antsPool == nil {
		return fmt.Errorf("shared ants pool is not available")
	}

	p := c.antsPool

	// Update each RedfishStatus basic status fields (PowerState, BmcStatus and healthy) concurrently
	for i := range redfishStatusList.Items {
		// Important: Create a new variable in the loop to avoid closure problems
		redfishStatus := redfishStatusList.Items[i]
		wg.Add(1)

		// Submit task to ants pool
		err := p.Submit(func() {
			defer wg.Done()
			c.log.Debugf("Updating basic status fields of RedfishStatus %s", redfishStatus.Name)
			if err := c.updateBasicStatus(&redfishStatus); err != nil {
				c.log.Errorf("Failed to update basic status of RedfishStatus %s: %v", redfishStatus.Name, err)
			}
		})

		if err != nil {
			c.log.Errorf("Failed to submit task for RedfishStatus %s: %v", redfishStatus.Name, err)
			wg.Done() // Reduce counter if submission failed
		}
	}

	// Wait for all tasks to complete
	wg.Wait()

	// Report pool statistics
	c.log.Debugf("RedfishStatus update completed. Pool stats: running=%d, free=%d, capacity=%d",
		p.Running(), p.Free(), p.Cap())
	return nil
}

// updateBasicStatus update single RedfishStatus basic status fields (PowerState, BmcStatus and healthy)
func (c *redfishStatusController) updateBasicStatus(oldRedfishStatus *topohubv1beta1.RedfishStatus) error {
	name := oldRedfishStatus.Name

	// lock resource to avoid concurrent update
	c.log.Debugf("Lock for updating basic status of RedfishStatus %s", name)
	lock := lock.LockManagerInstance.GetLock(name)
	lock.Lock()
	defer lock.Unlock()

	// create a copy
	if oldRedfishStatus.Status == nil {
		oldRedfishStatus.Status = &topohubv1beta1.RedfishStatusStatus{}
	}
	updated := oldRedfishStatus.DeepCopy()

	// get hostEndpoint
	hostEndpoint, err := c.getHostEndpoint(oldRedfishStatus)
	if err != nil {
		return fmt.Errorf("Failed to get hostEndpoint for RedfishStatus %s: %v", name, err)
	}

	// get connection data
	username, password, err := c.getSecretData(
		*hostEndpoint.Spec.SecretName,
		*hostEndpoint.Spec.SecretNamespace,
	)
	if err != nil {
		return fmt.Errorf("Failed to get secret data for HostEndpoint %s: %v", hostEndpoint.Name, err)
	}
	connInfo := &redfishstatusdata.RedfishConnectCon{
		Username: username,
		Password: password,
		IPAddr:   hostEndpoint.Spec.IPAddr,
		Port:     int(*hostEndpoint.Spec.Port),
		Http:     !*hostEndpoint.Spec.HTTPS,
	}

	// create redfish client
	var healthy bool
	client, err1 := redfish.NewClient(*connInfo, c.log)
	if err1 != nil {
		c.log.Warnf("Failed to create redfish client for RedfishStatus %s: %v", name, err1)
		healthy = false
	} else {
		healthy = true
	}

	defer func() {
		if client != nil {
			client.Logout()
		}
	}()

	// update healthy status
	updated.Status.Healthy = healthy

	// only update high frequency fields (PowerState and BmcStatus)
	if healthy {
		powerState, bmcStatus, err := client.GetBasicStatus()
		if err != nil {
			c.log.Warnf("Failed to get basic status for RedfishStatus %s: %v", name, err)
		} else {
			// update PowerState
			if updated.Status.Info == nil {
				updated.Status.Info = make(map[string]string)
			}
			updated.Status.Info["PowerState"] = powerState

			// update BmcStatus
			oldBmcStatus := ""
			if oldRedfishStatus.Status.Info != nil {
				oldBmcStatus = oldRedfishStatus.Status.Info["BmcStatus"]
			}
			if oldBmcStatus != bmcStatus {
				c.log.Infof("BmcStatus changed from %s to %s for RedfishStatus %s",
					oldBmcStatus, bmcStatus, name)
				updated.Status.Info["BmcStatus"] = bmcStatus
			}
		}
	}

	// compare and update status
	if !compareBasicStatus(updated.Status, oldRedfishStatus.Status) {
		c.log.Debugf("Basic status changed for RedfishStatus %s", name)
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		if err := c.client.Status().Update(context.Background(), updated); err != nil {
			return err
		}
		c.log.Infof("Successfully updated basic status for RedfishStatus %s", name)
	}

	return nil
}

// compareBasicStatus compare RedfishStatusStatus basic status fields (PowerState, BmcStatus and healthy)
func compareBasicStatus(a, b *topohubv1beta1.RedfishStatusStatus) bool {
	// if both are nil, they are equal
	if a == nil && b == nil {
		return true
	}
	// if one of them is nil, they are not equal
	if a == nil || b == nil {
		return false
	}

	// compare healthy status
	if a.Healthy != b.Healthy {
		return false
	}

	// compare PowerState
	aInfo := a.Info
	bInfo := b.Info
	if aInfo == nil && bInfo == nil {
		return true
	}
	if aInfo == nil || bInfo == nil {
		return false
	}

	// compare PowerState and BmcStatus
	if aInfo["PowerState"] != bInfo["PowerState"] {
		return false
	}
	if aInfo["BmcStatus"] != bInfo["BmcStatus"] {
		return false
	}

	return true
}

// Responsible for the first update of redfish information after redfishstatus creation
func (c *redfishStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.With("redfishstatus", req.Name)
	logger.Debugf("Reconciling RedfishStatus %s", req.Name)

	// Get RedfishStatus
	redfishStatus := &topohubv1beta1.RedfishStatus{}
	if err := c.client.Get(ctx, req.NamespacedName, redfishStatus); err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("RedfishStatus not found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get RedfishStatus")
		return ctrl.Result{}, nil
	}

	// return if redfishStatus.Status is not nil
	if redfishStatus.Status != nil {
		logger.Debugf("RedfishStatus %s has already been processed, skipping update", redfishStatus.Name)
		return ctrl.Result{}, nil
	}

	// update redfishStatus status
	if err := c.UpdateRedfishStatusInfoWrapper(redfishStatus); err != nil {
		logger.Error(err, "Failed to process RedfishStatus, will retry")
		return ctrl.Result{}, nil
	}

	logger.Debugf("Successfully processed RedfishStatus %s", redfishStatus.Name)
	return ctrl.Result{}, nil
}
