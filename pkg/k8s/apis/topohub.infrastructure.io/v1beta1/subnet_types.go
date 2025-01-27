package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="SUBNET",type="string",JSONPath=".spec.ipv4Subnet.subnet"
// +kubebuilder:printcolumn:name="TOTAL",type="integer",JSONPath=".status.IpTotalAmount"
// +kubebuilder:printcolumn:name="AVAILABLE",type="integer",JSONPath=".status.IpAvailableAmount"
// +kubebuilder:printcolumn:name="ASSIGNED",type="integer",JSONPath=".status.IpAssignAmount"
// +kubebuilder:printcolumn:name="RESERVED",type="integer",JSONPath=".status.IpReservedAmount"
// +kubebuilder:printcolumn:name="PXE",type="boolean",JSONPath=".spec.feature.enablePxe"
// +kubebuilder:printcolumn:name="ZTP",type="boolean",JSONPath=".spec.feature.enableZtp"
// +kubebuilder:subresource:status

// Subnet is the Schema for the subnets API
type Subnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetSpec   `json:"spec,omitempty"`
	Status SubnetStatus `json:"status,omitempty"`
}

// IPv4SubnetSpec defines the IPv4 subnet configuration
type IPv4SubnetSpec struct {
	// Subnet for DHCP server (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$`
	Subnet string `json:"subnet"`

	// IPRange for DHCP server (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}-([0-9]{1,3}\.){3}[0-9]{1,3}(,([0-9]{1,3}\.){3}[0-9]{1,3})*$`
	IPRange string `json:"ipRange"`

	// Gateway for DHCP server (optional)
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +optional
	Gateway *string `json:"gateway,omitempty"`

	// DNS server (optional)
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +optional
	Dns *string `json:"dns,omitempty"`
}

// InterfaceSpec defines the network interface configuration
type InterfaceSpec struct {
	// DHCP server interface (required)
	// +kubebuilder:validation:Required
	Interface string `json:"interface"`

	// VLAN ID (optional, 0-4094)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	// +optional
	VlanID *int32 `json:"vlanId,omitempty"`

	// Self IP for DHCP server (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$`
	IPv4 string `json:"ipv4"`
}

// FeatureSpec defines the feature configuration
type FeatureSpec struct {
	// EnableSyncEndpoint configuration
	// +optional
	EnableSyncEndpoint *EnableSyncEndpointSpec `json:"enableSyncEndpoint,omitempty"`

	// Enable DHCP IP binding
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	EnableBindDhcpIP bool `json:"enableBindDhcpIP"`

	// Enable reservation for non-DHCP IPs
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	EnableReserveNoneDhcpIP bool `json:"enableReserveNoneDhcpIP"`

	// Enable PXE boot support
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	EnablePxe bool `json:"enablePxe"`

	// Enable ZTP configuration for switch
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	EnableZtp bool `json:"enableZtp"`
}

// EnableSyncEndpointSpec defines the sync endpoint configuration
type EnableSyncEndpointSpec struct {
	// Enable DHCP client-based endpoint sync
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	DhcpClient bool `json:"dhcpClient"`

	// Enable subnet scan-based endpoint sync
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	ScanEndpoint bool `json:"scanEndpoint"`

	// Default cluster name
	// +optional
	DefaultClusterName *string `json:"defaultClusterName,omitempty"`

	// update what kind of endpoint
	// +kubebuilder:validation:Enum=hoststatus
	// +kubebuilder:validation:Required
	// +kubebuilder:default=hoststatus
	EndpointType string `json:"endpointType"`
}

// SubnetSpec defines the desired state of Subnet
type SubnetSpec struct {
	// IPv4Subnet configuration
	// +kubebuilder:validation:Required
	IPv4Subnet IPv4SubnetSpec `json:"ipv4Subnet"`

	// Interface configuration
	// +kubebuilder:validation:Required
	Interface InterfaceSpec `json:"interface"`

	// Feature configuration
	// +optional
	Feature *FeatureSpec `json:"feature,omitempty"`
}

// SubnetStatus defines the observed state of Subnet
type SubnetStatus struct {
	// Total number of IP addresses in the subnet
	DhcpIpTotalAmount int32 `json:"dhcpIpTotalAmount"`

	// Number of available IP addresses
	DhcpIpAvailableAmount int32 `json:"dhcpIpAvailableAmount"`

	// Number of assigned IP addresses
	DhcpIpAssignAmount int32 `json:"dhcpIpAssignAmount"`

	// Number of reserved IP addresses
	DhcpIpReservedAmount int32 `json:"dhcpIpReservedAmount"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubnetList contains a list of Subnet
type SubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnet `json:"items"`
}
