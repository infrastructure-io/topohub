package subnet

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"net"

	"github.com/infrastructure-io/topohub/pkg/config"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/tools"
)

// +kubebuilder:webhook:path=/validate-topohub-infrastructure-io-v1beta1-subnet,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=subnets,verbs=create;update,versions=v1beta1,name=vsubnet.kb.io,admissionReviewVersions=v1

// SubnetWebhook validates Subnet resources
type SubnetWebhook struct {
	Client client.Client
	config *config.AgentConfig
	log    *zap.SugaredLogger
}

func (w *SubnetWebhook) SetupWebhookWithManager(mgr ctrl.Manager, config config.AgentConfig) error {
	w.Client = mgr.GetClient()
	w.config = &config
	w.log = log.Logger.Named("subnetWebhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&topohubv1beta1.Subnet{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default implements webhook.Defaulter
func (w *SubnetWebhook) Default(ctx context.Context, obj runtime.Object) error {
	subnet, ok := obj.(*topohubv1beta1.Subnet)
	if !ok {
		return fmt.Errorf("object is not a Subnet")
	}

	w.log.Debugf("Setting initial values for nil fields in Subnet %s", subnet.Name)

	if subnet.Spec.Feature.EnableSyncEndpoint.DefaultClusterName != nil && *subnet.Spec.Feature.EnableSyncEndpoint.DefaultClusterName != "" {
		if subnet.ObjectMeta.Labels == nil {
			subnet.ObjectMeta.Labels = make(map[string]string)
		}
		subnet.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] = *subnet.Spec.Feature.EnableSyncEndpoint.DefaultClusterName
	} else {
		if subnet.ObjectMeta.Labels == nil {
			subnet.ObjectMeta.Labels = make(map[string]string)
		}
		subnet.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] = ""
	}

	if subnet.Spec.Interface.Interface == "" {
		subnet.Spec.Interface.Interface = w.config.DhcpServerInterface
	}
	if subnet.Spec.Interface.VlanID == nil {
		a := int32(0)
		subnet.Spec.Interface.VlanID = &a
	}

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *SubnetWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	subnet, ok := obj.(*topohubv1beta1.Subnet)
	if !ok {
		err := fmt.Errorf("object is not a Subnet")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Infof("Validating creation of Subnet %s", subnet.Name)

	if err := w.validateSubnet(ctx, subnet); err != nil {
		w.log.Errorf("Failed to validate Subnet %s: %v", subnet.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *SubnetWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldSubnet, ok := oldObj.(*topohubv1beta1.Subnet)
	if !ok {
		err := fmt.Errorf("old object is not a Subnet")
		w.log.Error(err.Error())
		return nil, err
	}

	newSubnet, ok := newObj.(*topohubv1beta1.Subnet)
	if !ok {
		err := fmt.Errorf("new object is not a Subnet")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Infof("Validating update of Subnet %s", newSubnet.Name)

	// 1. 验证 subnet 不允许修改
	if oldSubnet.Spec.IPv4Subnet.Subnet != newSubnet.Spec.IPv4Subnet.Subnet {
		return nil, fmt.Errorf("subnet %s cannot be modified", oldSubnet.Spec.IPv4Subnet.Subnet)
	}

	// 2. 验证 IP 范围只允许扩大，不允许缩小
	_, ipNet, err := net.ParseCIDR(newSubnet.Spec.IPv4Subnet.Subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet format: %v", err)
	}

	if err := tools.ValidateIPRangeExpansion(oldSubnet.Spec.IPv4Subnet.IPRange, newSubnet.Spec.IPv4Subnet.IPRange, ipNet); err != nil {
		return nil, err
	}

	// 3. 验证 interface name 不允许修改
	if oldSubnet.Spec.Interface.Interface != newSubnet.Spec.Interface.Interface {
		return nil, fmt.Errorf("interface name cannot be modified")
	}

	// 4. 验证 vlanId 不允许修改
	if !tools.Int32PtrEqual(oldSubnet.Spec.Interface.VlanID, newSubnet.Spec.Interface.VlanID) {
		return nil, fmt.Errorf("interface VLAN ID cannot be modified")
	}

	// 5. 验证 interface ipv4 不允许修改
	if oldSubnet.Spec.Interface.IPv4 != newSubnet.Spec.Interface.IPv4 {
		return nil, fmt.Errorf("interface IPv4 address cannot be modified")
	}

	// 执行其他常规验证
	if err := w.validateSubnet(ctx, newSubnet); err != nil {
		w.log.Errorf("Failed to validate Subnet %s: %v", newSubnet.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *SubnetWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateSubnet performs validation of the Subnet resource
func (w *SubnetWebhook) validateSubnet(ctx context.Context, subnet *topohubv1beta1.Subnet) error {
	// Parse and validate subnet first as it's needed for other validations
	_, ipNet, err := net.ParseCIDR(subnet.Spec.IPv4Subnet.Subnet)
	if err != nil {
		return fmt.Errorf("invalid subnet format: %v", err)
	}

	// Validate IP ranges are within subnet
	if err := tools.ValidateIPRange(subnet.Spec.IPv4Subnet.IPRange, ipNet); err != nil {
		return fmt.Errorf("invalid IP range: %v", err)
	}

	// Validate gateway is within subnet if specified
	if subnet.Spec.IPv4Subnet.Gateway != nil {
		gateway := net.ParseIP(*subnet.Spec.IPv4Subnet.Gateway)
		if gateway == nil {
			return fmt.Errorf("invalid gateway IP: %s", *subnet.Spec.IPv4Subnet.Gateway)
		}
		if !tools.ValidateIPInSubnet(gateway, ipNet) {
			return fmt.Errorf("gateway %s is not within subnet %s", *subnet.Spec.IPv4Subnet.Gateway, subnet.Spec.IPv4Subnet.Subnet)
		}
	}

	// Validate DNS if specified
	if subnet.Spec.IPv4Subnet.Dns != nil {
		dns := net.ParseIP(*subnet.Spec.IPv4Subnet.Dns)
		if dns == nil {
			return fmt.Errorf("invalid DNS IP: %s", *subnet.Spec.IPv4Subnet.Dns)
		}
	}

	// Validate interface configuration
	if err := w.validateInterface(&subnet.Spec.Interface, ipNet, subnet); err != nil {
		return fmt.Errorf("invalid interface configuration: %v", err)
	}

	return nil
}

// validateInterface validates the InterfaceSpec
func (w *SubnetWebhook) validateInterface(iface *topohubv1beta1.InterfaceSpec, cidr *net.IPNet, subnet *topohubv1beta1.Subnet) error {
	if iface == nil {
		return fmt.Errorf("interface spec is required")
	}

	// Validate interface exists on the system
	if err := tools.ValidateInterfaceExists(iface.Interface); err != nil {
		return err
	}

	// Validate VLAN ID if specified
	if iface.VlanID != nil {
		if *iface.VlanID < 0 || *iface.VlanID > 4094 {
			return fmt.Errorf("VLAN ID must be between 0 and 4094")
		}
	}

	// Validate interface IPv4 address is in the same subnet
	if err := tools.ValidateIPWithSubnetMatch(iface.IPv4, cidr); err != nil {
		return fmt.Errorf("interface IPv4 validation failed: %v", err)
	}

	// List all existing subnets to check for interface conflicts
	existingSubnets := &topohubv1beta1.SubnetList{}
	if err := w.Client.List(context.Background(), existingSubnets); err != nil {
		return fmt.Errorf("failed to list existing subnets: %v", err)
	}

	// Check for interface and VLAN ID conflicts
	for _, existingSubnet := range existingSubnets.Items {
		if existingSubnet.ObjectMeta.Name == subnet.ObjectMeta.Name {
			continue
		}

		// Check if using the same interface
		if existingSubnet.Spec.Interface.Interface == iface.Interface {
			// If both have VLAN IDs, check if they're the same
			if existingSubnet.Spec.Interface.VlanID != nil && iface.VlanID != nil {
				if *existingSubnet.Spec.Interface.VlanID == *iface.VlanID {
					return fmt.Errorf("interface %s with VLAN ID %d is already used by subnet %s",
						iface.Interface, *iface.VlanID, existingSubnet.Name)
				}
			} else if existingSubnet.Spec.Interface.VlanID == nil && iface.VlanID == nil {
				// If neither has VLAN ID, it's a conflict
				return fmt.Errorf("interface %s is already used by subnet %s",
					iface.Interface, existingSubnet.Name)
			} else if existingSubnet.Spec.Interface.VlanID == nil && iface.VlanID == nil && existingSubnet.Spec.Interface.Interface == iface.Interface {
				// If both have no VLAN ID, it's a conflict
				return fmt.Errorf("interface %s is already used by subnet %s",
					iface.Interface, existingSubnet.Name)
			}
			// If one has VLAN ID and the other doesn't, they can coexist
		}
	}

	return nil
}
