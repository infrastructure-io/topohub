package data

import (
	"github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// HostConnectCon 定义主机数据结构
type HostConnectCon struct {
	Info     *v1beta1.BasicInfo
	Username string
	Password string
	DhcpHost bool
}

// HostCache 定义主机缓存结构
type HostCache struct {
	lock lock.RWMutex
	data map[string]*HostConnectCon
}

var HostCacheDatabase *HostCache

func init() {
	HostCacheDatabase = &HostCache{
		data: make(map[string]*HostConnectCon),
	}
}

// Add 添加或更新缓存中的主机数据
func (c *HostCache) Add(name string, data HostConnectCon) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.data[name] = &data
}

// Delete 从缓存中删除指定主机数据
func (c *HostCache) Delete(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.data, name)
}

// Get 获取指定主机的数据
func (c *HostCache) Get(name string) *HostConnectCon {
	c.lock.RLock()
	defer c.lock.RUnlock()
	data, exists := c.data[name]
	if exists {
		t := *data
		return &t
	}
	return nil
}

// GetAll 返回缓存中的所有主机数据
func (c *HostCache) GetAll() map[string]HostConnectCon {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// 创建一个新的 map 来存储所有数据的副本
	result := make(map[string]HostConnectCon, len(c.data))
	for k, v := range c.data {
		result[k] = *v
	}

	return result
}

// GetDhcpClientInfo 返回缓存中的所有DHCP主机数据
func (c *HostCache) GetDhcpClientInfo() map[string]HostConnectCon {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// 创建一个新的 map 来存储所有数据的副本
	result := make(map[string]HostConnectCon, len(c.data))
	for k, v := range c.data {
		if v.DhcpHost {
			result[k] = *v
		}
	}

	return result
}

// GetStaticClientInfo 返回缓存中的所有静态主机数据
func (c *HostCache) GetStaticClientInfo() map[string]HostConnectCon {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// 创建一个新的 map 来存储所有数据的副本
	result := make(map[string]HostConnectCon, len(c.data))
	for k, v := range c.data {
		if !v.DhcpHost {
			result[k] = *v
		}
	}

	return result
}

func (c *HostCache) UpdateSecet(secretName, secretNamespace, username, password string) []string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	var changedHosts []string

	for name, v := range c.data {
		changed := false
		if v.Info.SecretName == secretName && v.Info.SecretNamespace == secretNamespace {
			if v.Username != username {
				v.Username = username
				changed = true
				log.Logger.Infof("update host status username for host %s", v.Info.IpAddr)
			}
			if v.Password != password {
				v.Password = password
				changed = true
				log.Logger.Infof("update host status password for host %s", v.Info.IpAddr)
			}
		}
		if changed {
			changedHosts = append(changedHosts, name)
		}
	}
	return changedHosts
}
