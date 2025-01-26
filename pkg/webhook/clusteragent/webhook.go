package clusteragent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"go.uber.org/zap"
)

// +kubebuilder:webhook:path=/validate-bmc-infrastructure-io-v1beta1-clusteragent,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=clusteragents,verbs=create;update,versions=v1beta1,name=vclusteragent.kb.io,admissionReviewVersions=v1

// ClusterAgentWebhook validates ClusterAgent resources
type ClusterAgentWebhook struct {
	Client client.Client
}

func (w *ClusterAgentWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&bmcv1beta1.ClusterAgent{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default implements webhook.Defaulter
func (w *ClusterAgentWebhook) Default(ctx context.Context, obj runtime.Object) error {
	clusterAgent, ok := obj.(*bmcv1beta1.ClusterAgent)
	if !ok {
		return fmt.Errorf("object is not a ClusterAgent")
	}

	logger := log.Logger.With(
		zap.String("webhook", "clusteragent"),
		zap.String("name", clusterAgent.Name),
		zap.String("ns", clusterAgent.Namespace),
		zap.String("action", "default"),
	)
	logger.Info("Setting initial values for nil fields in ClusterAgent")

	// Initialize AgentYaml fields
	if clusterAgent.Spec.AgentYaml.Image == "" {
		clusterAgent.Spec.AgentYaml.Image = ""
	}
	if clusterAgent.Spec.AgentYaml.UnderlayInterface == "" {
		clusterAgent.Spec.AgentYaml.UnderlayInterface = ""
	}
	if clusterAgent.Spec.AgentYaml.NodeName == "" {
		clusterAgent.Spec.AgentYaml.NodeName = ""
	}
	// hostNetwork defaults to false
	// No need to set it as bool defaults to false in Go

	if clusterAgent.Spec.AgentYaml.Replicas == nil {
		defaultReplicas := int32(0)
		clusterAgent.Spec.AgentYaml.Replicas = &defaultReplicas
	}

	// Initialize Endpoint if nil
	if clusterAgent.Spec.Endpoint == nil {
		clusterAgent.Spec.Endpoint = &bmcv1beta1.EndpointConfig{
			Port:            0,
			HTTPS:           false,
			SecretName:      "",
			SecretNamespace: "",
		}
	} else {
		if clusterAgent.Spec.Endpoint.SecretName == "" {
			clusterAgent.Spec.Endpoint.SecretName = ""
		}
		if clusterAgent.Spec.Endpoint.SecretNamespace == "" {
			clusterAgent.Spec.Endpoint.SecretNamespace = ""
		}
	}

	// Initialize Feature if nil
	if clusterAgent.Spec.Feature == nil {
		clusterAgent.Spec.Feature = &bmcv1beta1.FeatureConfig{
			EnableDhcpServer: false,
		}
	}

	// Initialize DhcpServerConfig if nil when EnableDhcpServer is true
	if clusterAgent.Spec.Feature.EnableDhcpServer && clusterAgent.Spec.Feature.DhcpServerConfig == nil {
		clusterAgent.Spec.Feature.DhcpServerConfig = &bmcv1beta1.DhcpServerConfig{
			EnableDhcpDiscovery: false,
			EnableBindDhcpIP:    false,
			EnableBindStaticIP:  false,
			DhcpServerInterface: "",
			Subnet:              "",
			IpRange:             "",
			Gateway:             "",
			SelfIp:              "",
		}
	}

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *ClusterAgentWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	clusterAgent, ok := obj.(*bmcv1beta1.ClusterAgent)
	if !ok {
		return nil, fmt.Errorf("object is not a ClusterAgent")
	}

	logger := log.Logger.With(
		zap.String("webhook", "clusteragent"),
		zap.String("name", clusterAgent.Name),
		zap.String("ns", clusterAgent.Namespace),
		zap.String("action", "validate-create"),
	)
	logger.Info("Validating ClusterAgent creation")

	if err := w.validateClusterAgent(ctx, clusterAgent); err != nil {
		logger.Errorf("Validation failed: %v", err)
		return nil, err
	}

	logger.Info("ClusterAgent validation successful")
	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *ClusterAgentWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	clusterAgent, ok := newObj.(*bmcv1beta1.ClusterAgent)
	if !ok {
		return nil, fmt.Errorf("object is not a ClusterAgent")
	}

	logger := log.Logger.With(
		zap.String("webhook", "clusteragent"),
		zap.String("name", clusterAgent.Name),
		zap.String("ns", clusterAgent.Namespace),
		zap.String("action", "validate-update"),
	)
	logger.Info("Validating ClusterAgent update")

	if err := w.validateClusterAgent(ctx, clusterAgent); err != nil {
		logger.Errorf("Validation failed: %v", err)
		return nil, err
	}

	logger.Info("ClusterAgent update validation successful")
	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *ClusterAgentWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (w *ClusterAgentWebhook) validateClusterAgent(ctx context.Context, clusterAgent *bmcv1beta1.ClusterAgent) error {
	logger := log.Logger.With(
		zap.String("webhook", "clusteragent"),
		zap.String("name", clusterAgent.Name),
		zap.String("ns", clusterAgent.Namespace),
		zap.String("action", "validate"),
	)

	// Validate required fields
	if clusterAgent.Name == "" {
		logger.Error("name is required")
		return fmt.Errorf("name is required")
	}
	if clusterAgent.Spec.AgentYaml.UnderlayInterface == "" {
		logger.Error("underlayInterface is required")
		return fmt.Errorf("underlayInterface is required")
	}

	// Validate name format
	if err := validateClusterName(clusterAgent.Name); err != nil {
		logger.Error(err.Error())
		return err
	}

	// Validate DHCP server configuration
	if clusterAgent.Spec.Feature != nil && clusterAgent.Spec.Feature.EnableDhcpServer {
		if clusterAgent.Spec.Feature.DhcpServerConfig == nil {
			logger.Error("dhcpServerConfig is required when enableDhcpServer is true")
			return fmt.Errorf("dhcpServerConfig is required when enableDhcpServer is true")
		}

		config := clusterAgent.Spec.Feature.DhcpServerConfig

		// Validate required fields
		if config.DhcpServerInterface == "" {
			logger.Error("dhcpServerInterface is required in dhcpServerConfig")
			return fmt.Errorf("dhcpServerInterface is required in dhcpServerConfig")
		}

		if config.Subnet == "" {
			logger.Error("subnet is required in dhcpServerConfig")
			return fmt.Errorf("subnet is required in dhcpServerConfig")
		}

		if config.IpRange == "" {
			logger.Error("ipRange is required in dhcpServerConfig")
			return fmt.Errorf("ipRange is required in dhcpServerConfig")
		}

		if config.Gateway == "" {
			logger.Error("gateway is required in dhcpServerConfig")
			return fmt.Errorf("gateway is required in dhcpServerConfig")
		}

		// Validate IP formats
		subnetRegex := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$`)
		if !subnetRegex.MatchString(config.Subnet) {
			logger.Error("invalid subnet format")
			return fmt.Errorf("invalid subnet format: %s", config.Subnet)
		}

		ipRangeRegex := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}-([0-9]{1,3}\.){3}[0-9]{1,3}$`)
		if !ipRangeRegex.MatchString(config.IpRange) {
			logger.Error("invalid ipRange format")
			return fmt.Errorf("invalid ipRange format: %s", config.IpRange)
		}

		ipRegex := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`)
		if !ipRegex.MatchString(config.Gateway) {
			logger.Error("invalid gateway format")
			return fmt.Errorf("invalid gateway format: %s", config.Gateway)
		}

		// Validate selfIp format (must be CIDR format)
		if config.SelfIp != "" {
			// Validate CIDR format
			if !subnetRegex.MatchString(config.SelfIp) {
				logger.Error("invalid selfIp format, must be in CIDR format (e.g., 192.168.0.2/24)")
				return fmt.Errorf("invalid selfIp format: %s, must be in CIDR format (e.g., 192.168.0.2/24)", config.SelfIp)
			}

			// Extract and compare subnet masks
			subnetMask, err := extractCIDRMask(config.Subnet)
			if err != nil {
				logger.Error("failed to extract mask from subnet")
				return fmt.Errorf("failed to extract mask from subnet: %v", err)
			}

			selfIpMask, err := extractCIDRMask(config.SelfIp)
			if err != nil {
				logger.Error("failed to extract mask from selfIp")
				return fmt.Errorf("failed to extract mask from selfIp: %v", err)
			}

			// Ensure masks are identical
			if subnetMask != selfIpMask {
				logger.Error("subnet and selfIp must have the same network mask")
				return fmt.Errorf("subnet (%s) and selfIp (%s) must have the same network mask", config.Subnet, config.SelfIp)
			}
		}
	}

	// Validate replicas
	if clusterAgent.Spec.AgentYaml.Replicas != nil && *clusterAgent.Spec.AgentYaml.Replicas < 0 {
		logger.Error("replicas must be greater than or equal to 0")
		return fmt.Errorf("replicas must be greater than or equal to 0")
	}

	logger.Info("ClusterAgent validation successful")
	return nil
}

func validateClusterName(name string) error {
	// Check if name is lowercase
	if !isLowerCase(name) {
		return fmt.Errorf("name must be lowercase: %s", name)
	}

	// Check if name is valid for Kubernetes resources
	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("invalid name %q: %s", name, errs[0])
	}

	return nil
}

func isLowerCase(s string) bool {
	return regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`).MatchString(s)
}

func extractCIDRMask(cidr string) (string, error) {
	parts := strings.Split(cidr, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid CIDR format: %s", cidr)
	}
	return parts[1], nil
}
