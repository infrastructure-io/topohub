package redfish

import (
	"context"

	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/log"
)

var redfishSessionPool pool.SessionPool[RedfishClient]

func InitSessionPool(ctx context.Context) {
	redfishSessionPool = pool.NewSessionPool(ctx, NewRedfishClientOperations(nil),
		pool.WithLogger(log.Logger.Named("sshSessionPool")))
}

func GetSessionPool() pool.SessionPool[RedfishClient] {
	return redfishSessionPool
}
