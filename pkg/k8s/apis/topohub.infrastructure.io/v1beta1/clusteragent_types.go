package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HostTypeDHCP     = "dhcp"
	HostTypeEndpoint = "hostEndpoint"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ready",type="boolean",JSONPath=".status.ready"

// ClusterAgent is the Schema for the clusteragents API
type ClusterAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAgentSpec   `json:"spec,omitempty"`
	Status ClusterAgentStatus `json:"status,omitempty"`
}

// ClusterAgentSpec defines the desired state of ClusterAgent
type ClusterAgentSpec struct {
	// AgentYaml contains the agent configuration
	// +kubebuilder:validation:Required
	AgentYaml AgentConfig `json:"agentYaml"`

	// Endpoint contains the endpoint configuration
	// +optional
	Endpoint *EndpointConfig `json:"endpoint,omitempty"`

	// Feature contains the feature configuration
	// +optional
	Feature *FeatureConfig `json:"feature,omitempty"`
}

// AgentConfig defines the configuration for the agent
type AgentConfig struct {
	// UnderlayInterface specifies the network interface configuration for underlay network
	// +optional
	UnderlayInterface string `json:"underlayInterface,omitempty"`

	// Image is the agent container image
	// +optional
	Image string `json:"image,omitempty"`

	// Replicas is the number of agents to run
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas"`

	// NodeAffinity defines scheduling constraints for the agent pods
	// +optional
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`

	// NodeName is a request to schedule this pod onto a specific node
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// HostNetwork indicates if the agent should use host network
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`
}

// EndpointConfig defines the endpoint configuration for the agent
type EndpointConfig struct {
	// Port is the endpoint port
	// +kubebuilder:default=443
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// SecretName is the name of the secret containing the TLS certificates
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// SecretNamespace is the namespace of the secret containing the TLS certificates
	// +optional
	SecretNamespace string `json:"secretNamespace,omitempty"`

	// HTTPS enables HTTPS for the endpoint
	// +kubebuilder:default=true
	HTTPS bool `json:"https,omitempty"`
}

// DhcpServerConfig defines the configuration for the DHCP server
type DhcpServerConfig struct {
	// EnableDhcpDiscovery enables DHCP discovery
	// +kubebuilder:default=true
	// +kubebuilder:validation:Required
	EnableDhcpDiscovery bool `json:"enableDhcpDiscovery"`

	// EnableBindDhcpIP enables DHCP IP binding
	// +kubebuilder:default=true
	EnableBindDhcpIP bool `json:"enableBindDhcpIP"`

	// EnableBindStaticIP enables static IP binding
	// +kubebuilder:default=true
	EnableBindStaticIP bool `json:"enableBindStaticIP"`

	// DhcpServerInterface specifies the interface for DHCP server
	// +kubebuilder:validation:Required
	DhcpServerInterface string `json:"dhcpServerInterface"`

	// Subnet specifies the subnet for DHCP server
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$`
	Subnet string `json:"subnet"`

	// IpRange specifies the IP range for DHCP server
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}-([0-9]{1,3}\.){3}[0-9]{1,3}$`
	IpRange string `json:"ipRange"`

	// Gateway specifies the gateway for DHCP server
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	Gateway string `json:"gateway"`

	// SelfIp specifies the self IP for DHCP server
	// +optional
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$`
	SelfIp string `json:"selfIp,omitempty"`
}

// FeatureConfig defines the feature configuration for the agent
type FeatureConfig struct {
	// EnableDhcpServer enables the DHCP server
	// +kubebuilder:default=true
	EnableDhcpServer bool `json:"enableDhcpServer,omitempty"`

	// DhcpServerConfig contains the DHCP server configuration
	// +optional
	DhcpServerConfig *DhcpServerConfig `json:"dhcpServerConfig,omitempty"`

}

// ClusterAgentStatus defines the observed state of ClusterAgent
type ClusterAgentStatus struct {
	// Whether the agent is ready
	// +kubebuilder:default=true
	Ready bool `json:"ready,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterAgentList contains a list of ClusterAgent
type ClusterAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAgent `json:"items"`
}
