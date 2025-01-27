package subnet

import (
	"github.com/infrastructure-io/topohub/pkg/config"
	ctrl "sigs.k8s.io/controller-runtime"
)

type SubnetManager interface {
	SetupWithManager(mgr ctrl.Manager) error
	Stop()
}

type subnetManager struct {
	config *config.AgentConfig
	mgr    ctrl.Manager
}

func NewSubnetReconciler(config config.AgentConfig) (SubnetManager, error) {
	return &subnetManager{
		config: &config,
	}, nil
}

func (s *subnetManager) SetupWithManager(mgr ctrl.Manager) error {
	return nil
}

func (s *subnetManager) Stop() {

}
