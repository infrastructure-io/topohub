package dhcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/tools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// statusUpdateWorker handles subnet status updates with rate limiting
func (s *dhcpServer) statusUpdateWorker() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var pendingUpdate bool

	for {
		select {
		case <-s.stopCh:
			s.log.Errorf("the status updater of subnet is exiting")
			return

		case <-s.statusUpdateCh:
			// Mark that we have a pending update, but don't process immediately
			pendingUpdate = true

		case <-ticker.C:
			// If we have a pending update when the ticker fires, process it
			if pendingUpdate {
				if err := s.updateSubnetWithRetry(); err != nil {
					s.log.Errorf("Failed to update subnet status: %v", err)
				}
				pendingUpdate = false
			}
		}
	}
}

// updateSubnetWithRetry updates subnet status with retries for conflicts
func (s *dhcpServer) updateSubnetWithRetry() error {
	backoff := wait.Backoff{
		Duration: time.Second,
		Factor:   2,
		Steps:    5,
	}

	return retry.OnError(backoff,
		func(err error) bool {
			// Retry on any error
			//return true
			// 这里我们只在遇到冲突错误时重试
			if errors.IsConflict(err) {
				s.log.Warnf("conflict occurred while updating subnet status, will retry")
				return true
			}
			s.log.Errorf("Abandon, failed to update subnet status: %v", err)
			return false
		},
		func() error {
			s.lockData.RLock()
			defer s.lockData.RUnlock()

			s.log.Debugf("it is about to update the status of subnet %s", s.subnet.Name)

			// 获取最新的 subnet
			current := &topohubv1beta1.Subnet{}
			if err := s.client.Get(context.Background(), types.NamespacedName{
				Name:      s.subnet.Name,
				Namespace: s.subnet.Namespace,
			}, current); err != nil {
				return err
			}

			// 统计 IP 使用情况
			totalIPs, err := tools.CountIPsInRange(s.subnet.Spec.IPv4Subnet.IPRange)
			if err != nil {
				s.log.Errorf("failed to count ips in range: %+v", err)
				totalIPs = 0
			}
			s.log.Debugf("total ip of dhcp server: %v", totalIPs)

			// 更新状态
			updated := current.DeepCopy()
			if updated.Status.DhcpStatus == nil {
				updated.Status.DhcpStatus = &topohubv1beta1.DhcpStatusSpec{}
			}

			// GetDhcpClient returns a string representation of all DHCP clients with their binding status
			updateClientFunc := func(dhcpClient, manualBindClients, autoBindClients map[string]*DhcpClientInfo) (string, uint64) {

				type clientInfo struct {
					Mac        string `json:"mac"`
					ManualBind bool   `json:"manualBind"`
					AutoBind   bool   `json:"autoBind"`
					Hostname   string `json:"hostname"`
				}

				clientMap := make(map[string]clientInfo)
				counter := uint64(0)

				// Add all current clients first
				for ip, client := range dhcpClient {
					clientMap[ip] = clientInfo{
						Mac:      client.MAC,
						AutoBind: false,
						Hostname: client.Hostname,
					}
					counter++
				}

				// Update or add bind clients
				for ip, client := range autoBindClients {
					if _, existed := clientMap[ip]; !existed {
						counter++
					}
					clientMap[ip] = clientInfo{
						Mac:        client.MAC,
						ManualBind: false,
						AutoBind:   true,
						Hostname:   client.Hostname,
					}
				}

				for ip, client := range manualBindClients {
					if _, existed := clientMap[ip]; !existed {
						counter++
					}
					clientMap[ip] = clientInfo{
						Mac:        client.MAC,
						ManualBind: true,
						AutoBind:   false,
						Hostname:   client.Hostname,
					}
				}

				if len(clientMap) == 0 {
					return "{}", 0
				}

				// Convert map to JSON string
				jsonBytes, err := json.Marshal(clientMap)
				if err != nil {
					s.log.Errorf("failed to marshal client map to JSON: %v", err)
					return "{}", 0
				}

				return string(jsonBytes), counter
			}
			clientDetails, usedIpAmount := updateClientFunc(s.currentLeaseClients, s.currentManualBindingClients, s.currentAutoBindingClients)
			updated.Status.DhcpClientDetails = clientDetails
			updated.Status.DhcpStatus.DhcpIpAvailableAmount = totalIPs - usedIpAmount
			updated.Status.DhcpStatus.DhcpIpTotalAmount = totalIPs
			updated.Status.DhcpStatus.DhcpIpActiveAmount = uint64(len(s.currentLeaseClients))
			updated.Status.DhcpStatus.DhcpIpManualBindAmount = uint64(len(s.currentManualBindingClients))
			updated.Status.DhcpStatus.DhcpIpAutoBindAmount = uint64(len(s.currentAutoBindingClients))
			updated.Status.DhcpStatus.DhcpIpBindAmount = updated.Status.DhcpStatus.DhcpIpManualBindAmount + updated.Status.DhcpStatus.DhcpIpAutoBindAmount

			if updated.Status.HostNode == nil || *updated.Status.HostNode != s.config.NodeName {
				s.log.Infof("update host node %s to subnet %s", s.config.NodeName, s.subnet.Name)
				updated.Status.HostNode = &s.config.NodeName
				// update Conditions
				if updated.Status.Conditions == nil {
					updated.Status.Conditions = []metav1.Condition{}
				}
				updated.Status.Conditions = append(updated.Status.Conditions, metav1.Condition{
					Type:               "DhcpServer",
					Reason:             "hostChange",
					Message:            "dhcp server is hosted by node " + s.config.NodeName,
					Status:             "True",
					LastTransitionTime: metav1.Now(),
				})
			}

			if reflect.DeepEqual(current.Status.DhcpStatus, updated.Status.DhcpStatus) {
				return nil
			}

			// 更新 crd 实例
			if err := s.client.Status().Update(context.Background(), updated); err != nil {
				s.log.Errorf("Failed to update subnet %s status: %v", s.subnet.Name, err)
				return err
			}
			s.log.Infof("succeeded to update subnet status for %s: %+v", updated.ObjectMeta.Name, updated.Status.DhcpStatus)
			return nil
		})
}
