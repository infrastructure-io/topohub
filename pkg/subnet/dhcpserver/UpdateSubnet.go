package dhcpserver

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// statusUpdateWorker handles subnet status updates with retries
func (s *dhcpServer) statusUpdateWorker() {
	for {
		select {
		case <-s.stopCh:
			s.log.Errorf("the status updater of subnet is exiting")
			return

		case subnet := <-s.statusUpdateCh:
			s.log.Errorf("it is about to update the subnet status: %+v", subnet.Status)
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

	return retry.OnError(backoff, 
		func(err error) bool {
			// Retry on any error
			return true
		},
		func() error {
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
