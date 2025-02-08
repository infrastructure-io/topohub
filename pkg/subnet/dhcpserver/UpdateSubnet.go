package dhcpserver

import (
	"context"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/tools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// statusUpdateWorker handles subnet status updates with retries
func (s *dhcpServer) statusUpdateWorker() {
	for {
		select {
		case <-s.stopCh:
			s.log.Errorf("the status updater of subnet is exiting")
			return

		case <-s.statusUpdateCh:
			s.log.Debugf("it is about to update the status of subnet %+v", s.subnet)
			if err := s.updateSubnetWithRetry(); err != nil {
				log.Logger.Errorf("Failed to update subnet status: %v", err)
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
			return errors.IsConflict(err)
		},
		func() error {
			s.lockData.RLock()
			defer s.lockData.RUnlock()

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
			updated.Status.DhcpStatus.DhcpIpTotalAmount = totalIPs
			updated.Status.DhcpStatus.DhcpIpAssignAmount = uint64(len(s.currentClients))
			updated.Status.DhcpStatus.DhcpIpAvailableAmount = totalIPs - uint64(len(s.currentClients))

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
			s.log.Infof("updated subnet status: %v", updated.Status.DhcpStatus)

			return nil
		})
}
