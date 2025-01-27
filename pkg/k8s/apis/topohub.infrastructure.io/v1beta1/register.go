// Package v1beta1 contains API Schema definitions for the bmc v1beta1 API group
// +kubebuilder:object:generate=true
// +groupName=topohub.infrastructure.io

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// API Group and Version constants
const (
	// GroupName is the group name used in this package
	GroupName = "topohub.infrastructure.io"
	// Version is the API version
	Version = "v1beta1"
	// APIVersion is the full API version string
	APIVersion = GroupName + "/" + Version
)

// Resource Kinds
const (
	// KindDhcpSubnet is the kind name for Subnet resource
	KindSubnet = "Subnet"

	// KindHostEndpoint is the kind name for HostEndpoint resource
	KindHostEndpoint = "HostEndpoint"

	// KindHostStatus is the kind name for HostStatus resource
	KindHostStatus = "HostStatus"

	// KindHostOperation is the kind name for HostOperation resource
	KindHostOperation = "HostOperation"
)

var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	AddToScheme = SchemeBuilder.AddToScheme
)

var (

	// Resource takes an unqualified resource and returns a Group qualified GroupResource
	Resource = func(resource string) schema.GroupResource {
		return SchemeGroupVersion.WithResource(resource).GroupResource()
	}

	// GroupResource takes an unqualified resource and returns a Group qualified GroupResource
	GroupResource = func(resource string) schema.GroupResource {
		return SchemeGroupVersion.WithResource(resource).GroupResource()
	}
)

func init() {
	SchemeBuilder.Register(&Subnet{}, &SubnetList{})
	SchemeBuilder.Register(&HostEndpoint{}, &HostEndpointList{})
	SchemeBuilder.Register(&HostStatus{}, &HostStatusList{})
	SchemeBuilder.Register(&HostOperation{}, &HostOperationList{})
}
