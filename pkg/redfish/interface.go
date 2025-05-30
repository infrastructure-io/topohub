package redfish

import (
	"fmt"
	"reflect"

	"github.com/stmcginnis/gofish/redfish"

	redfishstatusData "github.com/infrastructure-io/topohub/pkg/redfishstatus/data"
	"github.com/stmcginnis/gofish"
	"go.uber.org/zap"
)

// Client 定义了 Redfish 客户端接口
type RefishClient interface {
	Power(string) error
	GetInfo() (map[string]string, error)
	GetLog() ([]*redfish.LogEntry, error)
	GetSystemsLogEntries() ([]*redfish.LogEntry, error)
	GetManagersLogEntries() ([]*redfish.LogEntry, error)
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
func NewClient(hostCon interface{}, log *zap.SugaredLogger) (RefishClient, error) {
	var url string
	var ipAddr string
	var config gofish.ClientConfig

	if con, ok := hostCon.(redfishstatusData.RedfishConnectCon); ok {
		// 使用 buildEndpoint 或 buildRedfishEndpoint 函数，这里统一使用 buildRedfishEndpoint
		url = buildRedfishEndpoint(con)
		ipAddr = con.Info.IpAddr
		config = gofish.ClientConfig{
			Endpoint:         url,
			Username:         con.Username,
			Password:         con.Password,
			Insecure:         true,
			ReuseConnections: true,
		}
	} else {
		return nil, fmt.Errorf("unsupported connection type: %T", hostCon)
	}

	if c, ok := CacheClient[ipAddr]; ok {
		if reflect.DeepEqual(config, c.config) {
			_, err := c.client.Service.Systems()
			if err == nil {
				log.Debugf("use cached redfish client for %s", ipAddr)
				return c, nil
			}
		}
		log.Debugf("logout invalid cached redfish client for %s", ipAddr)
		c.client.Logout()
		delete(CacheClient, ipAddr)
	}

	log.Debugf("create new redfish client for %s", ipAddr)
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

	CacheClient[ipAddr] = c
	return c, nil
}

// buildRedfishEndpoint 根据 RedfishConnectCon 构建 Redfish 服务的端点 URL
func buildRedfishEndpoint(redfishCon redfishstatusData.RedfishConnectCon) string {
	protocol := "http"
	if redfishCon.Info.Https {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, redfishCon.Info.IpAddr, redfishCon.Info.Port)
}
