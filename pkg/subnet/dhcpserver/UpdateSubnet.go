package dhcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// UpdateService updates the subnet configuration and restarts the DHCP server
func (s *dhcpServer) UpdateService(subnet topohubv1beta1.Subnet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新 subnet
	s.subnet = &subnet

	// 重启 DHCP 服务
	if err := s.restartDhcpServer(); err != nil {
		return fmt.Errorf("failed to restart DHCP server: %v", err)
	}

	// 更新状态
	s.statusUpdateCh <- &subnet

	return nil
}

// statusUpdateWorker handles subnet status updates with retries
func (s *dhcpServer) statusUpdateWorker() {
	for {
		select {
		case <-s.stopCh:
			return
		case subnet := <-s.statusUpdateCh:
			if err := s.updateSubnetWithRetry(subnet); err != nil {
				log.Logger.Errorf("Failed to update subnet status: %v", err)
			}
		}
	}
}

// updateSubnetWithRetry updates subnet status with retries for conflicts
func (s *dhcpServer) updateSubnetWithRetry(subnet *topohubv1beta1.Subnet) error {
	backoff := wait.Backoff{
		Duration: time.Second,
		Factor:   2,
		Steps:    5,
	}

	return retry.OnError(backoff, func() error {
		// 获取最新的 subnet
		current := &topohubv1beta1.Subnet{}
		if err := s.client.Get(context.Background(), types.NamespacedName{
			Name:      subnet.Name,
			Namespace: subnet.Namespace,
		}, current); err != nil {
			return err
		}

		// 更新状态
		current.Status = subnet.Status
		if err := s.client.Status().Update(context.Background(), current); err != nil {
			return err
		}

		// 更新本地 subnet
		s.mu.Lock()
		s.subnet = current
		s.mu.Unlock()

		return nil
	})
}
