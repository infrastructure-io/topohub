package redfishstatus

import (
	"context"
	"fmt"
	"strings"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/subnet/dhcpserver"
	errors "k8s.io/apimachinery/pkg/api/errors"
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

// do the dhcp add event, create the hostendpoint, and the redfishstatus will be created by the hostendpoint controller
func (c *redfishStatusController) handleDHCPAdd(client dhcpserver.DhcpClientInfo) error {
	name := formatRedfishStatusName(client.IP)
	c.log.Debugf("Processing DHCP add event: %+v ", client)

	// check if the hostendpoint exists
	existingHostEndpoint := &topohubv1beta1.HostEndpoint{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existingHostEndpoint)
	if err == nil {
		c.log.Debugf("HostEndpoint %s already exists, using existing one", name)
		return nil
	}

	if !errors.IsNotFound(err) {
		c.log.Errorf("Failed to check if HostEndpoint %s exists: %v", name, err)
		return err
	}

	// the hostendpoint does not exist, create it
	c.log.Debugf("Creating new HostEndpoint %s for DHCP mode", name)
	// create the hostendpoint
	endpointType := topohubv1beta1.EndpointTypeRedfish
	port := int32(c.config.RedfishPort)

	hostEndpoint := &topohubv1beta1.HostEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelSourceType: topohubv1beta1.HostTypeDHCP,
				topohubv1beta1.LabelSubnetName: client.SubnetName,
			},
		},
		Spec: topohubv1beta1.HostEndpointSpec{
			Type:            &endpointType,
			ClusterName:     &client.ClusterName,
			IPAddr:          client.IP,
			HTTPS:           &c.config.RedfishHttps,
			Port:            &port,
			SecretName:      &c.config.RedfishSecretName,
			SecretNamespace: &c.config.RedfishSecretNamespace,
		},
	}
	if err := c.client.Create(context.Background(), hostEndpoint); err != nil {
		c.log.Errorf("Failed to create HostEndpoint %s: %v", name, err)
		return err
	}
	c.log.Infof("Successfully created HostEndpoint %s for DHCP mode", name)

	// create binding ip
	if c.createBindingIpForredfishstatus(client, hostEndpoint.GetUID()) {
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
	// update object
	if err := c.client.Update(context.Background(), updated); err != nil {
		c.log.Errorf("Failed to update labels of redfishstatus %s: %v", name, err)
		return err
	}
	c.log.Infof("Successfully disactivate the dhcp client of redfishstatus %s", name)

	return nil
}
