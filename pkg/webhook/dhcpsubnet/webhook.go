package dhcpsubnet

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

// +kubebuilder:webhook:path=/validate-bmc-infrastructure-io-v1beta1-dhcpsubnet,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=dhcpsubnets,verbs=create;update,versions=v1beta1,name=vdhcpsubnet.kb.io,admissionReviewVersions=v1

// DhcpSubnetWebhook validates DhcpSubnet resources
type DhcpSubnetWebhook struct {
	Client client.Client
}

func (w *DhcpSubnetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&topohubv1beta1.DhcpSubnet{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default implements webhook.Defaulter
func (w *DhcpSubnetWebhook) Default(ctx context.Context, obj runtime.Object) error {
	dhcpSubnet, ok := obj.(*topohubv1beta1.DhcpSubnet)
	if !ok {
		return fmt.Errorf("object is not a DhcpSubnet")
	}

	log.Logger.Infof("Setting initial values for nil fields in DhcpSubnet %s", dhcpSubnet.Name)
	falseValue := false
	trueValue := true

	// Set default values for Feature if not specified
	if dhcpSubnet.Spec.Feature == nil {
		dhcpSubnet.Spec.Feature = &topohubv1beta1.FeatureSpec{
			EnableBindDhcpIP:        &trueValue,
			EnableReserveNoneDhcpIP: &trueValue,
			EnablePxe:               &falseValue,
			EnableZtp:               &falseValue,
			EnableRedfish:           &falseValue,
			EnableSyncEndpoint: &topohubv1beta1.EnableSyncEndpointSpec{
				DhcpClient:   true,
				ScanEndpoint: false,
			},
		}
	} else {
		// Set default values for pointer fields if they are nil
		if dhcpSubnet.Spec.Feature.EnableBindDhcpIP == nil {
			dhcpSubnet.Spec.Feature.EnableBindDhcpIP = &trueValue
		}
		if dhcpSubnet.Spec.Feature.EnableReserveNoneDhcpIP == nil {
			dhcpSubnet.Spec.Feature.EnableReserveNoneDhcpIP = &trueValue
		}
		if dhcpSubnet.Spec.Feature.EnablePxe == nil {
			dhcpSubnet.Spec.Feature.EnablePxe = &falseValue
		}
		if dhcpSubnet.Spec.Feature.EnableZtp == nil {
			dhcpSubnet.Spec.Feature.EnableZtp = &falseValue
		}
		if dhcpSubnet.Spec.Feature.EnableRedfish == nil {
			dhcpSubnet.Spec.Feature.EnableRedfish = &falseValue
		}
		if dhcpSubnet.Spec.Feature.EnableSyncEndpoint == nil {
			dhcpSubnet.Spec.Feature.EnableSyncEndpoint = &topohubv1beta1.EnableSyncEndpointSpec{
				DhcpClient:   true,
				ScanEndpoint: false,
			}
		}
	}

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *DhcpSubnetWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dhcpSubnet, ok := obj.(*topohubv1beta1.DhcpSubnet)
	if !ok {
		return nil, fmt.Errorf("object is not a DhcpSubnet")
	}

	log.Logger.Infof("Validating creation of DhcpSubnet %s", dhcpSubnet.Name)

	if err := w.validateDhcpSubnet(ctx, dhcpSubnet); err != nil {
		log.Logger.Errorf("Failed to validate DhcpSubnet %s: %v", dhcpSubnet.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *DhcpSubnetWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	dhcpSubnet, ok := newObj.(*topohubv1beta1.DhcpSubnet)
	if !ok {
		return nil, fmt.Errorf("object is not a DhcpSubnet")
	}

	log.Logger.Infof("Validating update of DhcpSubnet %s", dhcpSubnet.Name)

	if err := w.validateDhcpSubnet(ctx, dhcpSubnet); err != nil {
		log.Logger.Errorf("Failed to validate DhcpSubnet %s: %v", dhcpSubnet.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *DhcpSubnetWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateDhcpSubnet performs validation of the DhcpSubnet resource
func (w *DhcpSubnetWebhook) validateDhcpSubnet(ctx context.Context, dhcpSubnet *topohubv1beta1.DhcpSubnet) error {

	if dhcpSubnet.Spec.Feature != nil {
		counter := 0
		if dhcpSubnet.Spec.FeatureEnablePxe != nil && *dhcpSubnet.Spec.FeatureEnablePxe {
			counter++
		}
		if dhcpSubnet.Spec.FeatureEnableZtp != nil && *dhcpSubnet.Spec.FeatureEnableZtp {
			counter++
		}
		if dhcpSubnet.Spec.FeatureEnableRedfish != nil && *dhcpSubnet.Spec.FeatureEnableRedfish {
			counter++
		}
		if counter > 1 {
			return fmt.Errorf("only one of EnablePxe, EnableZtp or EnableRedfish can be set to true")
		}
	}

	// Parse and validate subnet first as it's needed for other validations
	_, ipNet, err := net.ParseCIDR(dhcpSubnet.Spec.IPv4Subnet.Subnet)
	if err != nil {
		return fmt.Errorf("invalid subnet format: %v", err)
	}

	// Validate IP ranges are within subnet
	if err := ValidateIPRange(dhcpSubnet.Spec.IPv4Subnet.IPRange, ipNet); err != nil {
		return fmt.Errorf("invalid IP range: %v", err)
	}

	// Validate gateway is within subnet if specified
	if dhcpSubnet.Spec.IPv4Subnet.Gateway != nil {
		gateway := net.ParseIP(*dhcpSubnet.Spec.IPv4Subnet.Gateway)
		if gateway == nil {
			return fmt.Errorf("invalid gateway IP: %s", *dhcpSubnet.Spec.IPv4Subnet.Gateway)
		}
		if !ValidateIPInSubnet(gateway, ipNet) {
			return fmt.Errorf("gateway %s is not within subnet %s", *dhcpSubnet.Spec.IPv4Subnet.Gateway, dhcpSubnet.Spec.IPv4Subnet.Subnet)
		}
	}

	// Validate DNS if specified
	if dhcpSubnet.Spec.IPv4Subnet.Dns != nil {
		dns := net.ParseIP(*dhcpSubnet.Spec.IPv4Subnet.Dns)
		if dns == nil {
			return fmt.Errorf("invalid DNS IP: %s", *dhcpSubnet.Spec.IPv4Subnet.Dns)
		}
	}

	// Validate interface configuration
	if err := w.validateInterface(&dhcpSubnet.Spec.Interface, ipNet); err != nil {
		return fmt.Errorf("invalid interface configuration: %v", err)
	}

	return nil
}

// validateInterface validates the InterfaceSpec
func (w *DhcpSubnetWebhook) validateInterface(iface *topohubv1beta1.InterfaceSpec, subnet *net.IPNet) error {
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
