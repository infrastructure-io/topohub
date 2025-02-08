package hoststatus

import (
	"context"
	"reflect"
	"strings"
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
	log.Logger.Debugf("Processing DHCP add event: %+v ", client)

	// Try to get existing HostStatus
	existing := &topohubv1beta1.HostStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err == nil {
		// Create a copy of the existing object to avoid modifying the cache
		updated := existing.DeepCopy()

		// HostStatus exists, check if MAC changed,  or if failed to update status after creating
		if updated.Status.Basic.Mac != client.MAC {
			// MAC changed, update the object
			log.Logger.Infof("Updating HostStatus %s: MAC changed from %s to %s",
				name, updated.Status.Basic.Mac, client.MAC)
			updated.Status.Basic.Mac = client.MAC
		}
		expireTimeStr := client.DhcpExpireTime.Format(time.RFC3339)
		if updated.Status.Basic.DhcpExpireTime == nil || *updated.Status.Basic.DhcpExpireTime != expireTimeStr {
			oldTime := ""
			if updated.Status.Basic.DhcpExpireTime != nil {
				oldTime = *updated.Status.Basic.DhcpExpireTime
			}
			// DHCP expire time changed, update the object
			log.Logger.Infof("Updating HostStatus %s: DHCP ip %s expire time changed from %s to %s",
				name, &client.IP, oldTime, expireTimeStr)
			updated.Status.Basic.DhcpExpireTime = &expireTimeStr
		}

		if !reflect.DeepEqual(existing.Status, updated.Status) {
			updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
			if err := c.client.Status().Update(context.Background(), updated); err != nil {
				if errors.IsConflict(err) {
					log.Logger.Debugf("Conflict updating HostStatus %s, will retry", name)
					return err
				}
				log.Logger.Errorf("Failed to update HostStatus %s: %v", name, err)
				return err
			}
			log.Logger.Infof("Successfully updated HostStatus %s", name)
		}
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
				// topohubv1beta1.LabelIPAddr:       strings.Split(client.IP, "/")[0],
				// topohubv1beta1.LabelClientMode:   topohubv1beta1.HostTypeDHCP,
				// topohubv1beta1.LabelClientActive: "true",
				// topohubv1beta1.LabelClusterName:  client.ClusterName,
				topohubv1beta1.LabelSubnetName: client.SubnetName,
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
			DhcpExpireTime: func() *string {
				expireTimeStr := client.DhcpExpireTime.Format(time.RFC3339)
				return &expireTimeStr
			}(),
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

	// update the labels
	if hostStatus.ObjectMeta.Labels == nil {
		hostStatus.ObjectMeta.Labels = make(map[string]string)
	}
	// cluster name
	hostStatus.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] = hostStatus.Status.Basic.ClusterName
	// ip
	IpAddr := strings.Split(hostStatus.Status.Basic.IpAddr, "/")[0]
	hostStatus.ObjectMeta.Labels[topohubv1beta1.LabelIPAddr] = IpAddr
	// mode
	if hostStatus.Status.Basic.Type == topohubv1beta1.HostTypeDHCP {
		hostStatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeDHCP
	} else {
		hostStatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeEndpoint
	}
	// dhcp
	if hostStatus.Status.Basic.ActiveDhcpClient {
		hostStatus.ObjectMeta.Labels[topohubv1beta1.LabelClientActive] = "true"
	} else {
		hostStatus.ObjectMeta.Labels[topohubv1beta1.LabelClientActive] = "false"
	}

	if err := c.client.Status().Update(context.Background(), hostStatus); err != nil {
		log.Logger.Errorf("Failed to update status of HostStatus %s: %v", name, err)
		return err
	}

	log.Logger.Infof("Successfully created HostStatus %s", name)
	log.Logger.Debugf("DHCP client details - %+v", client)
	return nil
}

func (c *hostStatusController) handleDHCPDelete(client dhcpserver.DhcpClientInfo) error {
	name := formatHostStatusName(client.IP)
	log.Logger.Debugf("Processing DHCP delete event - %+v", client)

	// 获取现有的 HostStatus
	existing := &topohubv1beta1.HostStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Logger.Debugf("HostStatus %s not found, skip labeling", name)
			return nil
		}
		log.Logger.Errorf("Failed to get HostStatus %s: %v", name, err)
		return err
	}

	// 创建更新对象的副本
	updated := existing.DeepCopy()
	// 如果没有 labels map，则创建
	if updated.Labels == nil {
		updated.Labels = make(map[string]string)
	}
	// 添加或更新标签
	updated.Labels[topohubv1beta1.LabelClientActive] = "false"
	updated.Status.Basic.ActiveDhcpClient = false
	// 更新对象
	if err := c.client.Update(context.Background(), updated); err != nil {
		log.Logger.Errorf("Failed to update labels of HostStatus %s: %v", name, err)
		return err
	}
	log.Logger.Infof("Successfully disactivate the dhcp client of HostStatus %s", name)

	// log.Logger.Infof("Disable Bind DhcpIP, so delete the hoststatus - IP: %s, MAC: %s", client.IP, client.MAC)
	// existing := &topohubv1beta1.HostStatus{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: name,
	// 	},
	// }
	// if err := c.client.Delete(context.Background(), existing); err != nil {
	// 	if errors.IsNotFound(err) {
	// 		log.Logger.Debugf("HostStatus %s not found, already deleted", name)
	// 		return nil
	// 	}
	// 	log.Logger.Errorf("Failed to delete HostStatus %s: %v", name, err)
	// 	return err
	// }
	// log.Logger.Infof("Successfully deleted HostStatus %s", name)

	return nil
}
