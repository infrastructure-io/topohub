package data

import (
	"github.com/infrastructure-io/topohub/pkg/lock"
)

// BindingIPInfo 定义每一个 BindingIP 的信息
type BindingIPInfo struct {
	Subnet  string
	IPAddr  string
	MacAddr string
	Valid   bool
}

// BindingIPCache 定义绑定IP缓存结构
type BindingIPCache struct {
	lock lock.RWMutex
	data map[string]*BindingIPInfo
}

var BindingIPCacheDatabase *BindingIPCache

func init() {
	BindingIPCacheDatabase = &BindingIPCache{
		data: make(map[string]*BindingIPInfo),
	}
}

// Add 添加或更新缓存中的绑定IP数据
func (c *BindingIPCache) Add(name string, data BindingIPInfo) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.data[name] = &data
}

// Delete 从缓存中删除指定绑定IP数据
func (c *BindingIPCache) Delete(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.data, name)
}

// Get 获取指定绑定IP的数据
func (c *BindingIPCache) Get(name string) *BindingIPInfo {
	c.lock.RLock()
	defer c.lock.RUnlock()
	data, exists := c.data[name]
	if exists {
		t := *data
		return &t
	}
	return nil
}

// GetAll 返回缓存中的所有绑定IP数据
func (c *BindingIPCache) GetAll() map[string]BindingIPInfo {
	c.lock.RLock()
	defer c.lock.RUnlock()

	result := make(map[string]BindingIPInfo, len(c.data))
	for k, v := range c.data {
		result[k] = *v
	}

	return result
}

// GetBySubnet 返回指定子网下的所有绑定IP数据
func (c *BindingIPCache) GetBySubnet(subnet string) map[string]BindingIPInfo {
	c.lock.RLock()
	defer c.lock.RUnlock()

	result := make(map[string]BindingIPInfo, len(c.data))
	for k, v := range c.data {
		if v.Subnet == subnet {
			result[k] = *v
		}
	}

	return result
}
