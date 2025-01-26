package redfish

import (
	"fmt"
	"github.com/stmcginnis/gofish/redfish"
	"reflect"

	"github.com/infrastructure-io/topohub/pkg/agent/hoststatus/data"
	"github.com/stmcginnis/gofish"
	"go.uber.org/zap"
)

// Client 定义了 Redfish 客户端接口
type RefishClient interface {
	Power(string) error
	GetInfo() (map[string]string, error)
	GetLog() ([]*redfish.LogEntry, error)
}

// redfishClient 实现了 Client 接口
type redfishClient struct {
	config gofish.ClientConfig
	logger *zap.SugaredLogger
	client *gofish.APIClient
}

var _ RefishClient = (*redfishClient)(nil)

var CacheClient = make(map[string]*redfishClient)

// NewClient 创建一个新的 Redfish 客户端
func NewClient(hostCon data.HostConnectCon, log *zap.SugaredLogger) (RefishClient, error) {

	url := buildEndpoint(hostCon)
	config := gofish.ClientConfig{
		Endpoint:         url,
		Username:         hostCon.Username,
		Password:         hostCon.Password,
		Insecure:         true,
		ReuseConnections: true,
	}

	if c, ok := CacheClient[hostCon.Info.IpAddr]; ok {
		if reflect.DeepEqual(config, c.config) {
			_, err := c.client.Service.Systems()
			if err == nil {
				log.Debugf("use cached redfish client for %s", hostCon.Info.IpAddr)
				return c, nil
			}
		}
		log.Debugf("logout invalid cached redfish client for %s", hostCon.Info.IpAddr)
		c.client.Logout()
		delete(CacheClient, hostCon.Info.IpAddr)
	}

	log.Debugf("create new redfish client for %s", hostCon.Info.IpAddr)
	client, err := gofish.Connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %+v", err)
	}
	c := &redfishClient{
		config: config,
		logger: log.Named("redfish").With(
			zap.String("endpoint", url),
		),
		client: client,
	}

	CacheClient[hostCon.Info.IpAddr] = c
	return c, nil
}

// buildEndpoint 根据 HostConnectCon 构建 Redfish 服务的端点 URL
func buildEndpoint(hostCon data.HostConnectCon) string {
	protocol := "http"
	if hostCon.Info.Https {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, hostCon.Info.IpAddr, hostCon.Info.Port)
}
