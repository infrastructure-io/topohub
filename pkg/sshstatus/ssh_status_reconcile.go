// Complete the SSH information update for sshstatus

package sshstatus

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/clients/kube"
	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/clients/ssh"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/tools"
)

// UpdateSSHStatusInfo updates the SSH status information
func (c *sshStatusController) UpdateSSHStatusInfo(oldSSHStatus *topohubv1beta1.SSHStatus) error {
	name := oldSSHStatus.Name
	logger := c.log.With("name", name)
	// Acquire lock to update SSH status instance
	logger.Debugf("Lock for updating sshStatus")
	lock := lock.LockManagerInstance.GetLock(name)
	lock.Lock()
	defer lock.Unlock()

	// create a copy of sshStatus
	if oldSSHStatus.Status == nil {
		oldSSHStatus.Status = &topohubv1beta1.SSHStatusStatus{}
	}
	updated := oldSSHStatus.DeepCopy()

	// get hostEndpoint
	hostEndpoint, err := c.getHostEndpoinBySSHStatus(oldSSHStatus)
	if err != nil {
		return fmt.Errorf("failed to get hostEndpoint with sshStatus %s, err: %v", name, err)
	}

	// get connection data
	auth, err := kube.GetAuthenticationSecret(context.Background(), c.cacheReader,
		*hostEndpoint.Spec.SecretName, *hostEndpoint.Spec.SecretNamespace)
	if err != nil {
		return fmt.Errorf("failed to get secret data with HostEndpoint %s, err: %v", hostEndpoint.Name, err)
	}

	var healthy bool
	cfg := &ssh.SSHSessionConfig{
		Username:   auth.Username,
		Password:   auth.Password,
		IPAddr:     hostEndpoint.Spec.IPAddr,
		Port:       int(*hostEndpoint.Spec.Port),
		SSHKey:     auth.SSHKey,
		SSHKeyAuth: auth.SSHKey != "",
	}
	session, err := c.sshPool.GetOrCreate(cfg.SessionID(), cfg)
	if err != nil && err != pool.ErrSessionPingFailed {
		return fmt.Errorf("failed to get SSH session with sshStatus %s, err: %v", name, err)
	}
	if err == pool.ErrSessionPingFailed {
		logger.Warnf("Failed to ping SSH session, err: %v", err)
		healthy = false
	} else {
		healthy = true
	}
	// Check health status
	updated.Status.Healthy = healthy
	updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)

	// If healthy, get system information
	if healthy {
		client := session.GetClient()
		infoData, err := client.GetSystemInfo()
		if err != nil {
			c.log.Errorf("Failed to get info of SSHStatus: %v", err)
		} else {
			updated.Status.Info = infoData
		}
	}

	// update subnetName from range
	subnetName, err := tools.GetSubnetNameByIP(cfg.IPAddr, c.client, c.log)
	if err != nil {
		logger.Errorf("Failed to update subnet name from range: %v", err)
	} else {
		logger.Infof("Updated subnetName to %s for SSHStatus (IP: %s)", subnetName, cfg.IPAddr)
		updated.Status.Basic.SubnetName = &subnetName
	}

	// update clusterName
	if hostEndpoint.Spec.ClusterName != nil {
		updated.Status.Basic.ClusterName = *hostEndpoint.Spec.ClusterName
	}

	// Ensure status.info is never nil to avoid validation errors
	if updated.Status.Info == nil {
		updated.Status.Info = map[string]string{}
	}

	// If status hasn't changed, don't update
	if compareSSHStatus(updated.Status, oldSSHStatus.Status, c.log) {
		logger.Debug("SSHStatus has no changes, skipping update")
		return nil
	}

	// Update status
	logger.Debug("Updating SSHStatus")
	if err := c.client.Status().Update(context.Background(), updated); err != nil {
		if apierrors.IsConflict(err) {
			logger.Debug("Conflict updating SSHStatus, will retry")
			return err
		}
		logger.Errorf("Failed to update SSHStatus, err: %v", err)
		return err
	}

	logger.Debug("Successfully updated SSHStatus")
	return nil
}

// UpdateSSHStatusInfoWrapper updates the SSH status information for the specified name or all SSH statuses
func (c *sshStatusController) UpdateSSHStatusInfoWrapper(sshStatus *topohubv1beta1.SSHStatus) error {
	// get sshStatus list
	var sshStatusList topohubv1beta1.SSHStatusList
	modeinfo := ""
	listOpts := []client.ListOption{}
	// if status is nil, list all sshStatus
	if sshStatus == nil {
		if err := c.client.List(context.Background(), &sshStatusList, listOpts...); err != nil {
			c.log.Errorf("Failed to list SSHStatus: %v", err)
			return err
		}
	} else {
		sshStatusList.Items = append(sshStatusList.Items, *sshStatus)
	}

	// update each sshStatus
	for _, sshStatus := range sshStatusList.Items {
		c.log.Debugf("Updating SSHStatus %s", sshStatus.Name)
		if err := c.UpdateSSHStatusInfo(&sshStatus); err != nil {
			c.log.Errorf("Failed to update SSHStatus %s%s: %v",
				sshStatus.Name, modeinfo, err)
		}
	}
	return nil
}

// UpdateSSHStatusAtInterval periodically updates all SSH status information
func (c *sshStatusController) UpdateSSHStatusAtInterval() {
	interval := time.Duration(c.config.SSHStatusUpdateInterval) * time.Second
	if interval == 0 {
		interval = 60 * time.Second // Default to 60 seconds
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	c.log.Infof("begin to update all sshStatus at interval of %v seconds", interval/time.Second)

	for {
		select {
		case <-c.stopCh:
			c.log.Info("Stopping UpdateSSHStatusAtInterval")
			return
		case <-ticker.C:
			c.log.Debugf("update all sshStatus at interval")
			if err := c.UpdateSSHStatusInfoWrapper(nil); err != nil {
				c.log.Errorf("Failed to update SSH status: %v", err)
			}
		}
	}
}

func (c *sshStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.With("sshstatus", req.Name)
	logger.Debugf("Reconciling SSHStatus %s", req.Name)

	// Get sshStatus
	sshStatus := &topohubv1beta1.SSHStatus{}
	if err := c.client.Get(ctx, req.NamespacedName, sshStatus); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debugf("SSHStatus not found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get RedfishStatus")
		return ctrl.Result{}, nil
	}

	// return if sshStatus.Status is not nil
	if sshStatus.Status != nil {
		logger.Debugf("SSHStatus %s has already been processed, skipping update", sshStatus.Name)
		return ctrl.Result{}, nil
	}

	// update sshStatus status
	if err := c.UpdateSSHStatusInfoWrapper(sshStatus); err != nil {
		logger.Error(err, "Failed to process SSHStatus, will retry")
		return ctrl.Result{}, nil
	}

	logger.Debugf("Successfully processed SSHStatus %s", sshStatus.Name)
	return ctrl.Result{}, nil
}

func (c *sshStatusController) getHostEndpoinBySSHStatus(sshStatus *topohubv1beta1.SSHStatus) (*topohubv1beta1.HostEndpoint, error) {
	// all RedfishStatus should have ownerReferences
	if len(sshStatus.OwnerReferences) > 0 {
		for _, ownerRef := range sshStatus.OwnerReferences {
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

	return nil, fmt.Errorf("failed to get connection info for SSHStatus %s", sshStatus.Name)
}
