// Complete the SSH information update for sshstatus

package sshstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/lock"
	sshstatusdata "github.com/infrastructure-io/topohub/pkg/sshstatus/data"
	ssh "github.com/infrastructure-io/topohub/pkg/sshstatus/ssh"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ------------------------------  update the status.info of the sshstatus

// UpdateSSHStatusInfo updates the SSH status information
func (c *sshStatusController) UpdateSSHStatusInfo(name string, d *sshstatusdata.SSHConnectCon) (bool, error) {
	// Acquire lock to update SSH status instance
	c.log.Debugf("lock for updating sshStatus instance %s", name)
	lock := lock.LockManagerInstance.GetLock(name)
	lock.Lock()
	defer lock.Unlock()

	// Create SSH client
	var healthy bool
	client, err1 := ssh.NewClient(*d, c.log)
	if err1 != nil {
		c.log.Warnf("Failed to create SSH client for SSHStatus %s: %v", name, err1)
		healthy = false
	} else {
		defer client.Close()
		healthy = client.IsHealthy()
	}

	auth := "without username and password"
	if len(d.Username) != 0 && len(d.Password) != 0 {
		auth = "with username and password"
	} else if d.SSHKeyAuth && len(d.SSHKey) != 0 {
		auth = "with SSH key authentication"
	}
	c.log.Debugf("try to check SSH with url: %s:%d, %s", d.Info.IpAddr, d.Info.Port, auth)

	// Get existing SSHStatus
	existing := &topohubv1beta1.SSHStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err != nil {
		c.log.Errorf("Failed to get SSHStatus %s: %v", name, err)
		return false, err
	}
	updated := existing.DeepCopy()

	// If basic information is empty, populate it from SSHConnectCon
	if updated.Status.Basic.IpAddr == "" && d.Info != nil {
		c.log.Debugf("Populating empty basic information for SSHStatus %s from SSHConnectCon", name)
		updated.Status.Basic = *d.Info
	}

	// Check health status
	updated.Status.Healthy = healthy
	updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)

	// If healthy, get system information
	if healthy {
		infoData, err := client.GetSystemInfo()
		if err != nil {
			c.log.Errorf("Failed to get info of SSHStatus %s: %v", name, err)
			healthy = false
		} else {
			updated.Status.Info = infoData
		}
	}

	// If status hasn't changed, don't update
	if compareSSHStatus(updated.Status, existing.Status, c.log) {
		c.log.Debugf("SSHStatus %s has no changes, skipping update", name)
		return healthy, nil
	}

	// Update status
	c.log.Debugf("Updating SSHStatus %s", name)
	if err := c.client.Status().Update(context.Background(), updated); err != nil {
		if errors.IsConflict(err) {
			c.log.Debugf("Conflict updating SSHStatus %s, will retry", name)
			return healthy, err
		}
		c.log.Errorf("Failed to update SSHStatus %s: %v", name, err)
		return healthy, err
	}

	c.log.Infof("Successfully updated SSHStatus %s", name)
	return healthy, nil
}

// UpdateSSHStatusInfoWrapper updates the SSH status information for the specified name or all SSH statuses
func (c *sshStatusController) UpdateSSHStatusInfoWrapper(name string) error {
	if name != "" {
		// Update the specified SSH status
		d := sshstatusdata.SSHCacheDatabase.Get(name)
		if d == nil {
			c.log.Warnf("SSHStatus %s not found in cache", name)
			return fmt.Errorf("SSHStatus %s not found in cache", name)
		}

		_, err := c.UpdateSSHStatusInfo(name, d)
		if err != nil {
			c.log.Errorf("Failed to update SSHStatus %s: %v", name, err)
			return err
		}
	} else {
		// Update all SSH statuses
		hosts := sshstatusdata.SSHCacheDatabase.GetAll()
		c.log.Debugf("Updating all %d SSH statuses", len(hosts))

		for name, d := range hosts {
			_, err := c.UpdateSSHStatusInfo(name, &d)
			if err != nil {
				c.log.Errorf("Failed to update SSHStatus %s: %v", name, err)
			}
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
			if err := c.UpdateSSHStatusInfoWrapper(""); err != nil {
				c.log.Errorf("Failed to update SSH status: %v", err)
			}
		}
	}
}

// checks if the SSHStatus IP is in the Subnet's dhcpClientDetails and updates the subnetName
func (c *sshStatusController) cacheSSHStatusData(sshStatus *topohubv1beta1.SSHStatus, logger *zap.SugaredLogger) error {
	var (
		username, password, sshKey string
		sshKeyAuth                 bool
		err                        error
	)

	// check if the sshStatus data is in cache
	existingData := sshstatusdata.SSHCacheDatabase.Get(sshStatus.Name)
	if len(sshStatus.Status.Basic.SecretName) > 0 && len(sshStatus.Status.Basic.SecretNamespace) > 0 {
		username, password, sshKey, sshKeyAuth, err = c.getSecretData(
			sshStatus.Status.Basic.SecretName,
			sshStatus.Status.Basic.SecretNamespace,
		)
		if err != nil {
			logger.Errorf("Failed to get secret data for SSHStatus %s: %v", sshStatus.Name, err)
			return err
		}
		logger.Debugf("Preparing SSHStatus %s cache with username: %s, sshKeyAuth: %v",
			sshStatus.Name, username, sshKeyAuth)
	} else {
		logger.Debugf("Preparing SSHStatus %s cache with empty authentication", sshStatus.Name)
	}

	// create new connection object
	sshConnectCon := sshstatusdata.SSHConnectCon{
		Info:       &sshStatus.Status.Basic,
		Username:   username,
		Password:   password,
		SSHKey:     sshKey,
		SSHKeyAuth: sshKeyAuth,
	}

	// if the sshStatus data is in cache, check if the data is changed
	if existingData != nil {
		if reflect.DeepEqual(*existingData, sshConnectCon) {
			logger.Debugf("SSHStatus %s cache data unchanged, skipping update", sshStatus.Name)
			return nil
		}
	}

	// update cache
	sshstatusdata.SSHCacheDatabase.Add(sshStatus.Name, sshConnectCon)
	logger.Debugf("Successfully cached SSHStatus %s data (IP: %s)", sshStatus.Name, sshStatus.Status.Basic.IpAddr)
	return nil
}

// checks if the SSHStatus IP is in the Subnet's dhcpClientDetails and updates the subnetName
func (c *sshStatusController) updateSubnetNameFromDhcpClientDetails(sshStatus *topohubv1beta1.SSHStatus, logger *zap.SugaredLogger) error {
	if sshStatus == nil || sshStatus.Status.Basic.IpAddr == "" {
		return nil
	}

	targetIP := sshStatus.Status.Basic.IpAddr

	// Get all Subnet resources
	subnets := &topohubv1beta1.SubnetList{}
	if err := c.client.List(context.TODO(), subnets); err != nil {
		return fmt.Errorf("failed to list subnets: %v", err)
	}

	// Check dhcpClientDetails for each Subnet
	for _, subnet := range subnets.Items {
		if subnet.Status.DhcpClientDetails == "" {
			continue
		}

		// Parse dhcpClientDetails into map[string]interface{} where keys are IP addresses
		var dhcpClients map[string]interface{}
		if err := json.Unmarshal([]byte(subnet.Status.DhcpClientDetails), &dhcpClients); err != nil {
			logger.Warnf("Failed to unmarshal dhcpClientDetails for subnet %s: %v", subnet.Name, err)
			continue
		}

		// Check if target IP exists in the map keys
		if _, exists := dhcpClients[targetIP]; exists {
			// Found matching IP, update subnetName
			if sshStatus.Status.Basic.SubnetName == nil || *sshStatus.Status.Basic.SubnetName != subnet.Name {
				subnetName := subnet.Name
				sshStatus.Status.Basic.SubnetName = &subnetName
				logger.Infof("Updated subnetName to %s for SSHStatus %s (IP: %s)", subnet.Name, sshStatus.Name, targetIP)

				// Update SSHStatus resource
				if err := c.client.Status().Update(context.TODO(), sshStatus); err != nil {
					return fmt.Errorf("failed to update SSHStatus %s: %v", sshStatus.Name, err)
				}
			}
			return nil
		}
	}
	return nil
}

// ------------------------------  sshstatus reconcile, trigger updates

// processSSHStatus processes the SSH status, caches data, and updates status information
func (c *sshStatusController) processSSHStatus(sshStatus *topohubv1beta1.SSHStatus, logger *zap.SugaredLogger) error {
	// If IP address is empty and there are OwnerReferences, first get information from HostEndpoint
	if len(sshStatus.Status.Basic.IpAddr) == 0 && len(sshStatus.OwnerReferences) > 0 {
		// Try to find HostEndpoint owner
		for _, ownerRef := range sshStatus.OwnerReferences {
			if ownerRef.Kind == "HostEndpoint" {
				logger.Infof("Found HostEndpoint owner reference: %s", ownerRef.Name)

				// Get HostEndpoint
				hostEndpoint := &topohubv1beta1.HostEndpoint{}
				if err := c.client.Get(context.TODO(), client.ObjectKey{Name: ownerRef.Name}, hostEndpoint); err != nil {
					logger.Errorf("Failed to get HostEndpoint %s: %v", ownerRef.Name, err)
					return err
				}

				// Update SSHStatus with HostEndpoint information
				clusterName := ""
				if hostEndpoint.Spec.ClusterName != nil {
					clusterName = *hostEndpoint.Spec.ClusterName
				}

				sshStatus.Status = topohubv1beta1.SSHStatusStatus{
					Healthy:        false,
					LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
					Basic: topohubv1beta1.SSHBasicInfo{
						Type:        topohubv1beta1.HostTypeEndpoint,
						IpAddr:      hostEndpoint.Spec.IPAddr,
						Port:        *hostEndpoint.Spec.Port,
						ClusterName: clusterName,
					},
					Info: map[string]string{},
				}

				if hostEndpoint.Spec.SecretName != nil {
					sshStatus.Status.Basic.SecretName = *hostEndpoint.Spec.SecretName
				}
				if hostEndpoint.Spec.SecretNamespace != nil {
					sshStatus.Status.Basic.SecretNamespace = *hostEndpoint.Spec.SecretNamespace
				}
				break
			}
		}
	}

	logger.Debugf("Processing SSHStatus: %s (Type: %s, IP: %s, Health: %v)",
		sshStatus.Name,
		sshStatus.Status.Basic.Type,
		sshStatus.Status.Basic.IpAddr,
		sshStatus.Status.Healthy)

	if err := c.cacheSSHStatusData(sshStatus, logger); err != nil {
		logger.Errorf("Failed to cache SSHStatus data during processing: %v", err)
		return err
	}
	data := sshstatusdata.SSHCacheDatabase.Get(sshStatus.Name)
	if data == nil {
		return fmt.Errorf("no cache data found for SSHStatus %s after caching", sshStatus.Name)
	}

	sshConnectCon := *data

	// Check if IP is in Subnet's dhcpClientDetails and update subnetName
	if err := c.updateSubnetNameFromDhcpClientDetails(sshStatus, logger); err != nil {
		logger.Warnf("Failed to update subnet name from dhcp client details: %v", err)
	}

	// update sshStatus info
	_, err := c.UpdateSSHStatusInfo(sshStatus.Name, &sshConnectCon)
	if err != nil {
		logger.Errorf("Failed to update SSHStatus %s: %v", sshStatus.Name, err)
		return err
	}

	logger.Debugf("Successfully processed SSHStatus %s", sshStatus.Name)
	return nil
}

// Reconcile implements the reconcile.Reconciler interface
// Responsible for the first update of SSH information after sshstatus creation (subsequent updates are handled by UpdateSSHStatusAtInterval)
func (c *sshStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.With("sshstatus", req.Name)

	logger.Debugf("Reconciling SSHStatus %s", req.Name)

	// Get SSHStatus
	sshStatus := &topohubv1beta1.SSHStatus{}
	if err := c.client.Get(ctx, req.NamespacedName, sshStatus); err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("SSHStatus not found")
			data := sshstatusdata.SSHCacheDatabase.Get(req.Name)
			if data != nil {
				logger.Infof("delete sshStatus %s in cache, %+v", req.Name, *data)
				sshstatusdata.SSHCacheDatabase.Delete(req.Name)
			}
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SSHStatus")
		return ctrl.Result{}, err
	}

	if len(sshStatus.Status.Basic.IpAddr) != 0 {
		if err := c.cacheSSHStatusData(sshStatus, logger); err != nil {
			logger.Error(err, "Failed to cache SSHStatus data")
		}
		logger.Debugf("SSHStatus %s has IP address, skipping processing", sshStatus.Name)
		return ctrl.Result{}, nil
	}

	// Process SSHStatus (including getting basic information from OwnerReferences and updating status)
	if err := c.processSSHStatus(sshStatus, logger); err != nil {
		logger.Error(err, "Failed to process SSHStatus, will retry")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}
