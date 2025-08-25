package handler

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/clients/kube"
	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	"github.com/infrastructure-io/topohub/pkg/clients/ssh"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

var (
	// Global handler registry instance
	globalRegistry     map[string]HostEndpointHandler
	globalRegistryOnce sync.Once
)

type HostEndpointHandler interface {
	// RefreshSession refresh the session of the host endpoint.
	// If auth is nil, queries the Secret to obtain authentication information.
	RefreshSession(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint,
		auth *kube.AuthenticationSecret, logger *zap.SugaredLogger) error

	// CreateStatusIfNotExist create the status of the host endpoint if not exist.
	// Returns true if the status is created, false if it already exists.
	CreateStatusIfNotExist(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint,
		logger *zap.SugaredLogger) (bool, error)
}

func RegisterHostEndpointHandlers(
	cli client.Client,
	cache cache.Cache,
) map[string]HostEndpointHandler {
	return map[string]HostEndpointHandler{
		topohubv1beta1.EndpointTypeSSH: &SSHHostEndpointHandler{
			client:      cli,
			cacheReader: cache,
			sessionPool: ssh.GetSessionPool(),
		},
		topohubv1beta1.EndpointTypeRedfish: &RedfishHostEndpointHandler{
			client:      cli,
			cacheReader: cache,
			sessionPool: redfish.GetSessionPool(),
		},
	}
}

// GetHandlerRegistry returns the global handler registry instance.
func GetHandlerRegistry(cli client.Client, cacheReader cache.Cache) map[string]HostEndpointHandler {
	globalRegistryOnce.Do(func() {
		globalRegistry = RegisterHostEndpointHandlers(cli, cacheReader)
	})

	return globalRegistry
}
