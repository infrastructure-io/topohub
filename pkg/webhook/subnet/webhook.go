package subnet

import (
	"context"
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// +kubebuilder:webhook:path=/validate-bmc-infrastructure-io-v1beta1-subnet,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=subnets,verbs=create;update,versions=v1beta1,name=vsubnet.kb.io,admissionReviewVersions=v1

// SubnetWebhook validates Subnet resources
type SubnetWebhook struct {
	Client client.Client
}

func (w *SubnetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
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

	log.Logger.Infof("Setting initial values for nil fields in Subnet %s", subnet.Name)

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *SubnetWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	subnet, ok := obj.(*topohubv1beta1.Subnet)
	if !ok {
		return nil, fmt.Errorf("object is not a Subnet")
	}

	log.Logger.Infof("Validating creation of Subnet %s", subnet.Name)

	if err := w.validateSubnet(ctx, subnet); err != nil {
		log.Logger.Errorf("Failed to validate Subnet %s: %v", subnet.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *SubnetWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	subnet, ok := newObj.(*topohubv1beta1.Subnet)
	if !ok {
		return nil, fmt.Errorf("object is not a Subnet")
	}

	log.Logger.Infof("Validating update of Subnet %s", subnet.Name)

	if err := w.validateSubnet(ctx, subnet); err != nil {
		log.Logger.Errorf("Failed to validate Subnet %s: %v", subnet.Name, err)
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
	if err := ValidateIPRange(subnet.Spec.IPv4Subnet.IPRange, ipNet); err != nil {
		return fmt.Errorf("invalid IP range: %v", err)
	}

	// Validate gateway is within subnet if specified
	if subnet.Spec.IPv4Subnet.Gateway != nil {
		gateway := net.ParseIP(*subnet.Spec.IPv4Subnet.Gateway)
		if gateway == nil {
			return fmt.Errorf("invalid gateway IP: %s", *subnet.Spec.IPv4Subnet.Gateway)
		}
		if !ValidateIPInSubnet(gateway, ipNet) {
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
	if err := w.validateInterface(&subnet.Spec.Interface, ipNet); err != nil {
		return fmt.Errorf("invalid interface configuration: %v", err)
	}

	return nil
}

// validateInterface validates the InterfaceSpec
func (w *SubnetWebhook) validateInterface(iface *topohubv1beta1.InterfaceSpec, subnet *net.IPNet) error {
	// Validate interface name format
	if !IsValidInterfaceName(iface.Interface) {
		return fmt.Errorf("invalid interface name format: %s", iface.Interface)
	}

	// Validate interface exists on the system
	if err := ValidateInterfaceExists(iface.Interface); err != nil {
		return err
	}

	// Validate VLAN ID if specified
	if iface.VlanID != nil {
		if *iface.VlanID < 0 || *iface.VlanID > 4094 {
			return fmt.Errorf("VLAN ID must be between 0 and 4094")
		}
	}

	// Validate interface IPv4 address is in the same subnet
	if err := ValidateIPWithSubnetMatch(iface.IPv4, subnet); err != nil {
		return fmt.Errorf("interface IPv4 validation failed: %v", err)
	}

	return nil
}
