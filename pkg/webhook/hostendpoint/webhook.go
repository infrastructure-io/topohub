package hostendpoint

import (
	"context"
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:webhook:path=/validate-bmc-infrastructure-io-v1beta1-hostendpoint,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=hostendpoints,verbs=create;update,versions=v1beta1,name=vhostendpoint.kb.io,admissionReviewVersions=v1

// HostEndpointWebhook validates HostEndpoint resources
type HostEndpointWebhook struct {
	Client client.Client
}

func (w *HostEndpointWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&bmcv1beta1.HostEndpoint{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default implements webhook.Defaulter
func (w *HostEndpointWebhook) Default(ctx context.Context, obj runtime.Object) error {
	hostEndpoint, ok := obj.(*bmcv1beta1.HostEndpoint)
	if !ok {
		return fmt.Errorf("object is not a HostEndpoint")
	}

	log.Logger.Infof("Setting initial values for nil fields in HostEndpoint %s", hostEndpoint.Name)

	var clusterAgent *bmcv1beta1.ClusterAgent

	// Set default values
	if hostEndpoint.Spec.ClusterAgent == "" {
		// Try to set it to the only clusterAgent if there's only one
		var clusterAgentList bmcv1beta1.ClusterAgentList
		if err := w.Client.List(ctx, &clusterAgentList); err != nil {
			return fmt.Errorf("failed to list clusterAgents: %v", err)
		}
		if len(clusterAgentList.Items) == 1 {
			clusterAgent = &clusterAgentList.Items[0]
			hostEndpoint.Spec.ClusterAgent = clusterAgentList.Items[0].Name
			log.Logger.Infof("Setting default clusterAgent to %s for HostEndpoint %s", hostEndpoint.Spec.ClusterAgent, hostEndpoint.Name)
		}
	}

	if hostEndpoint.Spec.HTTPS == nil {
		defaultHTTPS := true
		hostEndpoint.Spec.HTTPS = &defaultHTTPS
		log.Logger.Infof("Setting default HTTPS to true for HostEndpoint %s", hostEndpoint.Name)
	}

	if hostEndpoint.Spec.Port == nil {
		defaultPort := int32(443)
		hostEndpoint.Spec.Port = &defaultPort
		log.Logger.Infof("Setting default Port to 443 for HostEndpoint %s", hostEndpoint.Name)
	}

	if (hostEndpoint.Spec.SecretName == nil || *hostEndpoint.Spec.SecretName == "") && (hostEndpoint.Spec.SecretNamespace == nil || *hostEndpoint.Spec.SecretNamespace == "") {
		var name, ns string
		if clusterAgent != nil {
			name = clusterAgent.Spec.Endpoint.SecretName
			ns = clusterAgent.Spec.Endpoint.SecretNamespace
		} else {
			err := w.Client.Get(ctx, client.ObjectKey{Name: hostEndpoint.Spec.ClusterAgent}, clusterAgent)
			if err != nil {
				return fmt.Errorf("failed to get clusterAgent: %v", err)
			}
			name = clusterAgent.Spec.Endpoint.SecretName
			ns = clusterAgent.Spec.Endpoint.SecretNamespace
		}
		hostEndpoint.Spec.SecretName = &name
		hostEndpoint.Spec.SecretNamespace = &ns
	}

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *HostEndpointWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	hostEndpoint, ok := obj.(*bmcv1beta1.HostEndpoint)
	if !ok {
		return nil, fmt.Errorf("object is not a HostEndpoint")
	}

	log.Logger.Infof("Validating creation of HostEndpoint %s", hostEndpoint.Name)

	if err := w.validateHostEndpoint(ctx, hostEndpoint); err != nil {
		log.Logger.Errorf("Failed to validate HostEndpoint %s: %v", hostEndpoint.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *HostEndpointWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	hostEndpoint, ok := newObj.(*bmcv1beta1.HostEndpoint)
	if !ok {
		return nil, fmt.Errorf("object is not a HostEndpoint")
	}

	log.Logger.Infof("Rejecting update of HostEndpoint %s: updates are not allowed", hostEndpoint.Name)
	return nil, fmt.Errorf("updates to HostEndpoint resources are not allowed")
}

// ValidateDelete implements webhook.Validator
func (w *HostEndpointWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (w *HostEndpointWebhook) validateHostEndpoint(ctx context.Context, hostEndpoint *bmcv1beta1.HostEndpoint) error {
	// Validate clusterAgent exists and is not empty
	if hostEndpoint.Spec.ClusterAgent == "" {
		return fmt.Errorf("clusterAgent cannot be empty")
	}

	clusterAgent := &bmcv1beta1.ClusterAgent{}
	if err := w.Client.Get(ctx, client.ObjectKey{Name: hostEndpoint.Spec.ClusterAgent}, clusterAgent); err != nil {
		return fmt.Errorf("clusterAgent %s not found", hostEndpoint.Spec.ClusterAgent)
	}

	// Validate IP address is in subnet
	ip := net.ParseIP(hostEndpoint.Spec.IPAddr)
	if ip == nil {
		return fmt.Errorf("invalid IP address, it should be like 192.168.0.10 ")
	}

	// if clusterAgent.Spec.Feature == nil || clusterAgent.Spec.Feature.DhcpServerConfig == nil {
	// 	return fmt.Errorf("spec.DhcpServerConfig not found in clusterAgent %s", clusterAgent.Name)
	// }

	// _, subnet, err := net.ParseCIDR(clusterAgent.Spec.Feature.DhcpServerConfig.Subnet)
	// if err != nil {
	// 	return fmt.Errorf("invalid DhcpServerConfig.Subnet %q in clusterAgent %s: %v", clusterAgent.Spec.Feature.DhcpServerConfig.Subnet, clusterAgent.Name, err)
	// }

	// if !subnet.Contains(ip) {
	// 	return fmt.Errorf("IP address %s is not in clusterAgent DhcpServerConfig.Subnet %s", hostEndpoint.Spec.IPAddr, clusterAgent.Spec.Feature.DhcpServerConfig.Subnet)
	// }

	// Check for IP address uniqueness
	var existingHostEndpoints bmcv1beta1.HostEndpointList
	if err := w.Client.List(ctx, &existingHostEndpoints); err != nil {
		return fmt.Errorf("failed to list hostEndpoints: %v", err)
	}

	for _, existing := range existingHostEndpoints.Items {
		if existing.Name != hostEndpoint.Name && existing.Spec.IPAddr == hostEndpoint.Spec.IPAddr {
			return fmt.Errorf("IP address %s is already in use by another hostEndpoint %q", hostEndpoint.Spec.IPAddr, existing.Name)
		}
	}

	// Check IP address conflict with existing HostStatus
	hostStatusList := &bmcv1beta1.HostStatusList{}
	if err := w.Client.List(ctx, hostStatusList); err != nil {
		return fmt.Errorf("failed to list HostStatus: %v", err)
	}

	for _, hostStatus := range hostStatusList.Items {
		if hostStatus.Status.Basic.IpAddr == hostEndpoint.Spec.IPAddr {
			return fmt.Errorf("IP address %s is already used by HostStatus %s", hostEndpoint.Spec.IPAddr, hostStatus.Name)
		}
	}

	// Validate secret if both secretName and secretNamespace are provided
	if (hostEndpoint.Spec.SecretName != nil && *hostEndpoint.Spec.SecretName != "") && (hostEndpoint.Spec.SecretNamespace != nil && *hostEndpoint.Spec.SecretNamespace != "") {
		secret := &corev1.Secret{}
		if err := w.Client.Get(ctx, client.ObjectKey{
			Name:      *hostEndpoint.Spec.SecretName,
			Namespace: *hostEndpoint.Spec.SecretNamespace,
		}, secret); err != nil {
			return fmt.Errorf("secret %s/%s not found", *hostEndpoint.Spec.SecretNamespace, *hostEndpoint.Spec.SecretName)
		}

		if _, ok := secret.Data["username"]; !ok {
			return fmt.Errorf("secret must contain username key")
		}
		if _, ok := secret.Data["password"]; !ok {
			return fmt.Errorf("secret must contain password key")
		}
	}

	setName := false
	setNs := false
	if hostEndpoint.Spec.SecretName != nil && *hostEndpoint.Spec.SecretName != "" {
		setName = true
	}
	if hostEndpoint.Spec.SecretNamespace != nil && *hostEndpoint.Spec.SecretNamespace != "" {
		setNs = true
	}
	if (setName && !setNs) || (!setName && setNs) {
		return fmt.Errorf("secretName and secretNamespace must be both set or both unset")
	}

	return nil
}
