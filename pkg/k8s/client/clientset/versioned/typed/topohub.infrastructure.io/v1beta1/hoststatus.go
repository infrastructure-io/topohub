// Copyright 2024 Authors of infrastructure-io
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	context "context"

	topohubinfrastructureiov1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	scheme "github.com/infrastructure-io/topohub/pkg/k8s/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// HostStatusesGetter has a method to return a HostStatusInterface.
// A group's client should implement this interface.
type HostStatusesGetter interface {
	HostStatuses() HostStatusInterface
}

// HostStatusInterface has methods to work with HostStatus resources.
type HostStatusInterface interface {
	Create(ctx context.Context, hostStatus *topohubinfrastructureiov1beta1.HostStatus, opts v1.CreateOptions) (*topohubinfrastructureiov1beta1.HostStatus, error)
	Update(ctx context.Context, hostStatus *topohubinfrastructureiov1beta1.HostStatus, opts v1.UpdateOptions) (*topohubinfrastructureiov1beta1.HostStatus, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, hostStatus *topohubinfrastructureiov1beta1.HostStatus, opts v1.UpdateOptions) (*topohubinfrastructureiov1beta1.HostStatus, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*topohubinfrastructureiov1beta1.HostStatus, error)
	List(ctx context.Context, opts v1.ListOptions) (*topohubinfrastructureiov1beta1.HostStatusList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *topohubinfrastructureiov1beta1.HostStatus, err error)
	HostStatusExpansion
}

// hostStatuses implements HostStatusInterface
type hostStatuses struct {
	*gentype.ClientWithList[*topohubinfrastructureiov1beta1.HostStatus, *topohubinfrastructureiov1beta1.HostStatusList]
}

// newHostStatuses returns a HostStatuses
func newHostStatuses(c *TopohubV1beta1Client) *hostStatuses {
	return &hostStatuses{
		gentype.NewClientWithList[*topohubinfrastructureiov1beta1.HostStatus, *topohubinfrastructureiov1beta1.HostStatusList](
			"hoststatuses",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *topohubinfrastructureiov1beta1.HostStatus { return &topohubinfrastructureiov1beta1.HostStatus{} },
			func() *topohubinfrastructureiov1beta1.HostStatusList {
				return &topohubinfrastructureiov1beta1.HostStatusList{}
			},
		),
	}
}
