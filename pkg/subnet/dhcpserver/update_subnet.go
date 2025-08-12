package dhcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/tools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type clientInfo struct {
	Mac            string `json:"mac"`
	IsBound        bool   `json:"isBound"`
	IsAllocated    bool   `json:"isAllocated"`
	Hostname       string `json:"hostname"`
	DhcpExpireTime string `json:"dhcpExpireTime"`
}

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

			clientDetails, usedIpAmount := updateClientFunc(s.log, s.currentLeaseClients, s.currentManualBindingClients)
			updated.Status.DhcpClientDetails = clientDetails
			updated.Status.DhcpStatus.DhcpIpAvailableAmount = totalIPs - usedIpAmount
			updated.Status.DhcpStatus.DhcpIpTotalAmount = totalIPs
			updated.Status.DhcpStatus.DhcpIpActiveAmount = uint64(len(s.currentLeaseClients))
			updated.Status.DhcpStatus.DhcpIpBindAmount = uint64(len(s.currentManualBindingClients))

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

			if reflect.DeepEqual(current.Status, updated.Status) {
				return nil
			}

			// 更新 crd 实例
			if err := s.client.Status().Update(context.Background(), updated); err != nil {
				s.log.Errorf("Failed to update subnet %s status: %v", s.subnet.Name, err)
				return err
			}
			s.log.Infof("succeeded to update subnet status for %s: %+v", updated.Name, updated.Status.DhcpStatus)
			return nil
		})
}

// updateClientFunc returns a string representation of all DHCP clients with their binding status
// and the count of used IP addresses
func updateClientFunc(log *zap.SugaredLogger, dhcpClient, manualBindClients map[string]*DhcpClientInfo) (string, uint64) {
	clientMap := make(map[string]clientInfo)
	counter := uint64(0)

	// first add all manual bind clients
	var expireTime string
	for ip, client := range manualBindClients {
		expireTime = ""
		if !client.DhcpExpireTime.IsZero() {
			expireTime = client.DhcpExpireTime.Format(time.RFC3339)
		}
		clientMap[ip] = clientInfo{
			Mac:            client.MAC,
			IsBound:        true,
			IsAllocated:    false,
			Hostname:       client.Hostname,
			DhcpExpireTime: expireTime,
		}
		counter++
	}
	// then add all dhcp clients
	for ip, client := range dhcpClient {
		// if the client is already in manual bind clients, update it
		if existing, ok := clientMap[ip]; ok {
			// if mac is not the same, output error
			if existing.Mac != client.MAC {
				log.Errorf("ip %s is already bound to mac %s, but now mac %s", ip, existing.Mac, client.MAC)
			}
			existing.Mac = client.MAC
			existing.IsAllocated = true
			existing.Hostname = client.Hostname
			existing.DhcpExpireTime = client.DhcpExpireTime.Format(time.RFC3339)
			clientMap[ip] = existing
		} else {
			clientMap[ip] = clientInfo{
				Mac:            client.MAC,
				IsBound:        false,
				IsAllocated:    true,
				Hostname:       client.Hostname,
				DhcpExpireTime: client.DhcpExpireTime.Format(time.RFC3339),
			}
			counter++
		}
	}

	if len(clientMap) == 0 {
		return "{}", 0
	}

	// Convert map to JSON string
	jsonBytes, err := json.Marshal(clientMap)
	if err != nil {
		log.Errorf("failed to marshal client map to JSON: %v", err)
		return "{}", 0
	}

	return string(jsonBytes), counter
}
