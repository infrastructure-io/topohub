package ssh

import (
	"context"

	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/log"
)

var sshSessionPool pool.SessionPool[Client]

func InitSessionPool(ctx context.Context) {
	sshSessionPool = pool.NewSessionPool(ctx, NewSSHCLientOperations(nil),
		pool.WithLogger(log.Logger.Named("sshSessionPool")))
}

func GetSessionPool() pool.SessionPool[Client] {
	return sshSessionPool
}
