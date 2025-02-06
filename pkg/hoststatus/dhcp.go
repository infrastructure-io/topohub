package hoststatus

import (
	"context"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/subnet/dhcpserver"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// retryDelay is the delay before retrying a failed operation
	retryDelay = time.Second
)

func shouldRetry(err error) bool {
	return errors.IsConflict(err) || errors.IsServerTimeout(err) || errors.IsTooManyRequests(err)
}

// DHCP manager 把 dhcp client 事件告知后，进行 hoststatus 更新
func (c *hostStatusController) processDHCPEvents() {

	for {
		select {
		case <-c.stopCh:
			log.Logger.Errorf("Stopping processDHCPEvents")
			return
		case event := <-c.addChan:
			if err := c.handleDHCPAdd(event); err != nil {
				if shouldRetry(err) {
					log.Logger.Debugf("Retrying DHCP add event for IP %s after %v due to: %v",
						event.IP, retryDelay, err)
					go func(e dhcpserver.DhcpClientInfo) {
						time.Sleep(retryDelay)
						c.addChan <- e
					}(event)
				}
			}
		case event := <-c.deleteChan:
			if err := c.handleDHCPDelete(event); err != nil {
				if shouldRetry(err) {
					log.Logger.Debugf("Retrying DHCP delete event for IP %s after %v due to: %v",
						event.IP, retryDelay, err)
					go func(e dhcpserver.DhcpClientInfo) {
						time.Sleep(retryDelay)
						c.deleteChan <- e
					}(event)
				}
			}
		}
	}
}

// create the hoststatus for the dhcp client
func (c *hostStatusController) handleDHCPAdd(client dhcpserver.DhcpClientInfo) error {

	name := formatHostStatusName(client.IP)
	log.Logger.Debugf("Processing DHCP add event - IP: %s, MAC: %s, Active: %v, Lease: %s -> %s, ClusterName: %s, Subnet: %s",
		client.IP, client.MAC, client.Active, client.StartTime, client.ClusterName, client.Subnet)

	// Try to get existing HostStatus
	existing := &topohubv1beta1.HostStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err == nil {
		// HostStatus exists, check if MAC changed,  or if failed to update status after creating
		if existing.Status.Basic.Mac == client.MAC {
			log.Logger.Debugf("HostStatus %s exists with same MAC %s, no update needed", name, client.MAC)
			return nil
		}
		// MAC changed, update the object
		log.Logger.Infof("Updating HostStatus %s: MAC changed from %s to %s",
			name, existing.Status.Basic.Mac, client.MAC)

		// Create a copy of the existing object to avoid modifying the cache
		updated := existing.DeepCopy()
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		updated.Status.Basic.Mac = client.MAC

		if err := c.client.Status().Update(context.Background(), updated); err != nil {
			if errors.IsConflict(err) {
				log.Logger.Debugf("Conflict updating HostStatus %s, will retry", name)
				return err
			}
			log.Logger.Errorf("Failed to update HostStatus %s: %v", name, err)
			return err
		}
		log.Logger.Infof("Successfully updated HostStatus %s", name)
		log.Logger.Debugf("Updated DHCP client details - IP: %s, MAC: %s, Lease: %s ",
			client.IP, client.MAC, client.StartTime)
		return nil
	}

	if !errors.IsNotFound(err) {
		log.Logger.Errorf("Failed to get HostStatus %s: %v", name, err)
		return err
	}

	hostStatus := &topohubv1beta1.HostStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelIPAddr:       client.IP,
				topohubv1beta1.LabelClientMode:   topohubv1beta1.HostTypeDHCP,
				topohubv1beta1.LabelClientActive: "true",
				topohubv1beta1.LabelClusterName:  client.ClusterName,
				topohubv1beta1.LabelSubnet:       client.Subnet,
			},
		},
	}
	log.Logger.Debugf("Creating new HostStatus %s", name)

	// HostStatus doesn't exist, create new one
	// IMPORTANT: When creating a new HostStatus, we must follow a two-step process:
	// 1. First create the resource with only metadata (no status). This is because
	//    the Kubernetes API server does not allow setting status during creation.
	// 2. Then update the status separately using UpdateStatus. If we try to set
	//    status during creation, the status will be silently ignored, leading to
	//    a HostStatus without any status information until the next reconciliation.
	if err := c.client.Create(context.Background(), hostStatus); err != nil {
		log.Logger.Errorf("Failed to create HostStatus %s: %v", name, err)
		return err
	}

	// Get the latest version of the resource after creation
	// if err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, hostStatus); err != nil {
	// 	log.Logger.Errorf("Failed to get latest version of HostStatus %s: %v", name, err)
	// 	return err
	// }

	// Now update the status using the latest version
	hostStatus.Status = topohubv1beta1.HostStatusStatus{
		Healthy:        false,
		LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
		Basic: topohubv1beta1.BasicInfo{
			Type:             topohubv1beta1.HostTypeDHCP,
			IpAddr:           client.IP,
			Mac:              client.MAC,
			Port:             int32(c.config.RedfishPort),
			Https:            c.config.RedfishHttps,
			ActiveDhcpClient: true,
			ClusterName:      client.ClusterName,
		},
		Info: map[string]string{},
		Log: topohubv1beta1.LogStruct{
			TotalLogAccount:   0,
			WarningLogAccount: 0,
			LastestLog:        nil,
			LastestWarningLog: nil,
		},
	}
	if c.config.RedfishSecretName != "" {
		hostStatus.Status.Basic.SecretName = c.config.RedfishSecretName
	}
	if c.config.RedfishSecretNamespace != "" {
		hostStatus.Status.Basic.SecretNamespace = c.config.RedfishSecretNamespace
	}

	if err := c.client.Status().Update(context.Background(), hostStatus); err != nil {
		log.Logger.Errorf("Failed to update status of HostStatus %s: %v", name, err)
		return err
	}

	log.Logger.Infof("Successfully created HostStatus %s", name)
	log.Logger.Debugf("DHCP client details - IP: %s, MAC: %s, Lease: %s",
		client.IP, client.MAC, client.StartTime)
	return nil
}

func (c *hostStatusController) handleDHCPDelete(client dhcpserver.DhcpClientInfo) error {
	name := formatHostStatusName(client.IP)
	log.Logger.Debugf("Processing DHCP delete event - IP: %s, MAC: %s", client.IP, client.MAC)

	// if c.config.AgentObjSpec.Feature.DhcpServerConfig.EnableBindDhcpIP {
	// 	log.Logger.Infof("Enable Bind DhcpIP, so just label the hoststatus - IP: %s, MAC: %s", client.IP, client.MAC)

	// 	// 获取现有的 HostStatus
	// 	existing := &topohubv1beta1.HostStatus{}
	// 	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	// 	if err != nil {
	// 		if errors.IsNotFound(err) {
	// 			log.Logger.Debugf("HostStatus %s not found, skip labeling", name)
	// 			return nil
	// 		}
	// 		log.Logger.Errorf("Failed to get HostStatus %s: %v", name, err)
	// 		return err
	// 	}

	// 	// 创建更新对象的副本
	// 	updated := existing.DeepCopy()
	// 	// 如果没有 labels map，则创建
	// 	if updated.Labels == nil {
	// 		updated.Labels = make(map[string]string)
	// 	}
	// 	// 添加或更新标签
	// 	updated.Labels[topohubv1beta1.LabelClientActive] = "false"
	// 	updated.Status.Basic.ActiveDhcpClient = false
	// 	// 更新对象
	// 	if err := c.client.Update(context.Background(), updated); err != nil {
	// 		log.Logger.Errorf("Failed to update labels of HostStatus %s: %v", name, err)
	// 		return err
	// 	}
	// 	log.Logger.Infof("Successfully labeled HostStatus %s with dhcp-bound=false", name)

	// } else {
	log.Logger.Infof("Disable Bind DhcpIP, so delete the hoststatus - IP: %s, MAC: %s", client.IP, client.MAC)

	existing := &topohubv1beta1.HostStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := c.client.Delete(context.Background(), existing); err != nil {
		if errors.IsNotFound(err) {
			log.Logger.Debugf("HostStatus %s not found, already deleted", name)
			return nil
		}
		log.Logger.Errorf("Failed to delete HostStatus %s: %v", name, err)
		return err
	}
	log.Logger.Infof("Successfully deleted HostStatus %s", name)
	//}

	return nil
}
