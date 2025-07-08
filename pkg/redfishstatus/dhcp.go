package redfishstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/subnet/dhcpserver"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// retryDelay is the delay before retrying a failed operation
	retryDelay = time.Second

	// BasicInfoAnnotation is the annotation key for storing basicInfo as JSON
	BasicInfoAnnotation = "topohub.infrastructure.io/basic-info"
)

func shouldRetry(err error) bool {
	return errors.IsConflict(err) || errors.IsServerTimeout(err) || errors.IsTooManyRequests(err)
}

// processDHCPEvents processes DHCP events from the DHCP manager
func (c *redfishStatusController) processDHCPEvents() {

	for {
		select {
		case <-c.stopCh:
			c.log.Infof("Stopping processDHCPEvents")
			return
		case event := <-c.addChan:
			if err := c.handleDHCPAdd(event); err != nil {
				if shouldRetry(err) {
					c.log.Debugf("Retrying DHCP add event for IP %s after %v due to: %v",
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
					c.log.Debugf("Retrying DHCP delete event for IP %s after %v due to: %v",
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

func (c *redfishStatusController) createBindingIpForredfishstatus(client dhcpserver.DhcpClientInfo, ownerUid types.UID) (retry bool) {
	name := formatRedfishStatusName(client.IP)

	// creat bindingIp for the redfishstatus
	if client.EnableBindIpForRedfishstatus == nil || !*client.EnableBindIpForRedfishstatus {
		c.log.Infof("do not need to bind ip for redfishstatus %s", name)
		return false
	}

	c.log.Debugf("checking to create bindip %s for redfishstatus %s", name, name)
	setTrue := true
	bindingIP := topohubv1beta1.BindingIp{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelRedfishStatus: name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         topohubv1beta1.APIVersion,
					Kind:               topohubv1beta1.KindredfishStatus,
					Name:               name,
					UID:                ownerUid,
					BlockOwnerDeletion: &setTrue,
				},
			},
		},
		Spec: topohubv1beta1.BindingIpSpec{
			IpAddr:  client.IP,
			MacAddr: client.MAC,
			Subnet:  client.SubnetName,
		},
	}

	ctx := context.Background()
	bindingIPList := &topohubv1beta1.BindingIpList{}
	if err := c.client.List(ctx, bindingIPList); err != nil {
		c.log.Errorf("Failed to list BindingIPs: %v", err)
		return true
	}
	for _, existingBindingIP := range bindingIPList.Items {
		if existingBindingIP.Spec.IpAddr == bindingIP.Spec.IpAddr && strings.EqualFold(existingBindingIP.Spec.MacAddr, bindingIP.Spec.MacAddr) {
			c.log.Debugf("bindingip %s already exists for host %s: %+v", existingBindingIP.Name, name, existingBindingIP.Spec)
			return false
		}

		if existingBindingIP.Name == bindingIP.Name {
			c.log.Errorf("A conflicted bindgIp already exists for host %s: existed=%+v, expected=%+v", bindingIP.Name, existingBindingIP.Spec, bindingIP.Spec)
			// ignore binding ip
			return false
		}

		if existingBindingIP.Spec.IpAddr == bindingIP.Spec.IpAddr {
			c.log.Errorf("BindingIP %s already bind IP %s with mac %s, expected mac %s", bindingIP.Name, bindingIP.Spec.IpAddr, existingBindingIP.Spec.MacAddr, bindingIP.Spec.MacAddr)
			return false
		}
	}

	// create the bindingip
	if err := c.client.Create(context.Background(), &bindingIP); err != nil {
		c.log.Errorf("Failed to create BindingIP: %v", err)
		return true
	}

	c.log.Infof("created bindingip %s for redfishstatus %s: %+v", bindingIP.Name, name, bindingIP)

	return false
}

// create the redfishstatus for the dhcp client
func (c *redfishStatusController) handleDHCPAdd(client dhcpserver.DhcpClientInfo) error {

	name := formatRedfishStatusName(client.IP)
	c.log.Debugf("Processing DHCP add event: %+v ", client)

	// Try to get existing redfishstatus
	existing := &topohubv1beta1.RedfishStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err == nil {
		// Create a copy of the existing object to avoid modifying the cache
		updated := existing.DeepCopy()

		// redfishstatus exists, check if MAC changed,  or if failed to update status after creating
		if updated.Status.Basic.Mac != client.MAC {
			// MAC changed, update the object
			c.log.Infof("Updating redfishstatus %s: MAC changed from %s to %s",
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
			c.log.Infof("Updating redfishstatus %s: DHCP ip %s expire time changed from %s to %s",
				name, &client.IP, oldTime, expireTimeStr)
			updated.Status.Basic.DhcpExpireTime = &expireTimeStr
		}

		if !reflect.DeepEqual(existing.Status, updated.Status) {
			updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
			if err := c.client.Status().Update(context.Background(), updated); err != nil {
				if errors.IsConflict(err) {
					c.log.Debugf("Conflict updating redfishstatus %s, will retry", name)
					return err
				}
				c.log.Errorf("Failed to update redfishstatus %s: %v", name, err)
				return err
			}
			c.log.Infof("Successfully updated redfishstatus %s", name)
		}

		// make sure the binding ip
		if c.createBindingIpForredfishstatus(client, existing.GetUID()) {
			return fmt.Errorf("failed to create binding ip for redfishstatus %s: %+v", name, client)
		}

		return nil
	}

	if !errors.IsNotFound(err) {
		c.log.Errorf("Failed to get redfishstatus %s: %v", name, err)
		return err
	}

	// check connecting to the host
	c.log.Debugf("checking connecting to the redfishstatus %s", client.IP)
	basicInfo := topohubv1beta1.BasicInfo{
		Type:             topohubv1beta1.HostTypeDHCP,
		IpAddr:           client.IP,
		Mac:              client.MAC,
		Port:             int32(c.config.RedfishPort),
		Https:            c.config.RedfishHttps,
		ActiveDhcpClient: true,
		ClusterName:      client.ClusterName,
		SubnetName:       &client.SubnetName,
		DhcpExpireTime: func() *string {
			expireTimeStr := client.DhcpExpireTime.Format(time.RFC3339)
			return &expireTimeStr
		}(),
		Hostname:        &client.Hostname,
		SecretName:      c.config.RedfishSecretName,
		SecretNamespace: c.config.RedfishSecretNamespace,
	}

	c.log.Debugf("succeed to checking the redfishstatus %s, and create redfishstatus for it", client.IP)
	redfishstatus := &topohubv1beta1.RedfishStatus{
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
	c.log.Debugf("Creating new redfishstatus %s", name)

	// redfishstatus doesn't exist, create new one
	// write basicInfo to annotation
	if redfishstatus.ObjectMeta.Annotations == nil {
		redfishstatus.ObjectMeta.Annotations = make(map[string]string)
	}

	basicInfoJSON, err := json.Marshal(basicInfo)
	if err != nil {
		c.log.Errorf("Failed to marshal basicInfo for redfishstatus %s: %v", name, err)
		return err
	}
	redfishstatus.ObjectMeta.Annotations[BasicInfoAnnotation] = string(basicInfoJSON)

	// update the labels
	if redfishstatus.ObjectMeta.Labels == nil {
		redfishstatus.ObjectMeta.Labels = make(map[string]string)
	}
	// cluster name
	redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] = basicInfo.ClusterName
	// ip
	IpAddr := strings.Split(basicInfo.IpAddr, "/")[0]
	redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelIPAddr] = IpAddr
	// mode
	redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeDHCP
	// dhcp
	redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelClientActive] = "true"

	// create resource
	if err := c.client.Create(context.Background(), redfishstatus); err != nil {
		c.log.Errorf("Failed to create redfishstatus %s: %v", name, err)
		return err
	}

	c.log.Infof("Successfully created redfishstatus %s", name)
	c.log.Debugf("DHCP client details - %+v", client)

	if c.createBindingIpForredfishstatus(client, redfishstatus.GetUID()) {
		return fmt.Errorf("failed to create binding ip for redfishstatus %s: %+v", name, client)
	}

	return nil
}

func (c *redfishStatusController) handleDHCPDelete(client dhcpserver.DhcpClientInfo) error {
	name := formatRedfishStatusName(client.IP)
	c.log.Debugf("Processing DHCP delete event - %+v", client)

	// get the redfishstatus
	existing := &topohubv1beta1.RedfishStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			c.log.Debugf("redfishstatus %s not found, skip labeling", name)
			return nil
		}
		c.log.Errorf("Failed to get redfishstatus %s: %v", name, err)
		return err
	}

	// create update object
	updated := existing.DeepCopy()
	// if no labels map, create
	if updated.Labels == nil {
		updated.Labels = make(map[string]string)
	}
	// update labels
	updated.Labels[topohubv1beta1.LabelClientActive] = "false"
	updated.Status.Basic.ActiveDhcpClient = false
	// update object
	if err := c.client.Update(context.Background(), updated); err != nil {
		c.log.Errorf("Failed to update labels of redfishstatus %s: %v", name, err)
		return err
	}
	c.log.Infof("Successfully disactivate the dhcp client of redfishstatus %s", name)

	// log.Logger.Infof("Disable Bind DhcpIP, so delete the redfishstatus - IP: %s, MAC: %s", client.IP, client.MAC)
	// existing := &topohubv1beta1.redfishstatus{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: name,
	// 	},
	// }
	// if err := c.client.Delete(context.Background(), existing); err != nil {
	// 	if errors.IsNotFound(err) {
	// 		log.Logger.Debugf("redfishstatus %s not found, already deleted", name)
	// 		return nil
	// 	}
	// 	log.Logger.Errorf("Failed to delete redfishstatus %s: %v", name, err)
	// 	return err
	// }
	// log.Logger.Infof("Successfully deleted redfishstatus %s", name)

	return nil
}
