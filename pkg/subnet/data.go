package subnet

import (
	"github.com/infrastructure-io/topohub/pkg/lock"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// SubnetCache represents a thread-safe cache for Subnet instances
type SubnetCache struct {
	mu      lock.RWMutex
	subnets map[string]*topohubv1beta1.Subnet
}

// NewSubnetCache creates a new SubnetCache instance
func NewSubnetCache() *SubnetCache {
	return &SubnetCache{
		subnets: make(map[string]*topohubv1beta1.Subnet),
	}
}

// Get returns a subnet by its name
func (c *SubnetCache) Get(name string) (*topohubv1beta1.Subnet, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	subnet, exists := c.subnets[name]
	return subnet, exists
}

// GetAllNames returns a slice of all subnet names in the cache
func (c *SubnetCache) GetAllNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var names []string
	for name := range c.subnets {
		names = append(names, name)
	}
	return names
}

// Set stores a subnet in the cache
func (c *SubnetCache) Set(subnet *topohubv1beta1.Subnet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subnets[subnet.Name] = subnet.DeepCopy()
}

// Delete removes a subnet from the cache
func (c *SubnetCache) Delete(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subnets, name)
}

// HasSpecChanged checks if the spec of a subnet has changed compared to the cached version
func (c *SubnetCache) HasSpecChanged(subnet *topohubv1beta1.Subnet) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.subnets[subnet.Name]
	if !exists {
		return true
	}

	// Deep equal comparison of the specs
	return !specEqual(cached.Spec, subnet.Spec)
}

// specEqual compares two SubnetSpecs for equality
func specEqual(a, b topohubv1beta1.SubnetSpec) bool {
	// Compare IPv4Subnet
	if a.IPv4Subnet.Subnet != b.IPv4Subnet.Subnet ||
		a.IPv4Subnet.IPRange != b.IPv4Subnet.IPRange {
		return false
	}

	// Compare optional fields
	if (a.IPv4Subnet.Gateway == nil) != (b.IPv4Subnet.Gateway == nil) {
		return false
	}
	if a.IPv4Subnet.Gateway != nil && b.IPv4Subnet.Gateway != nil &&
		*a.IPv4Subnet.Gateway != *b.IPv4Subnet.Gateway {
		return false
	}

	if (a.IPv4Subnet.Dns == nil) != (b.IPv4Subnet.Dns == nil) {
		return false
	}
	if a.IPv4Subnet.Dns != nil && b.IPv4Subnet.Dns != nil &&
		*a.IPv4Subnet.Dns != *b.IPv4Subnet.Dns {
		return false
	}

	// Compare Interface
	if a.Interface.Interface != b.Interface.Interface ||
		a.Interface.IPv4 != b.Interface.IPv4 {
		return false
	}

	if (a.Interface.VlanID == nil) != (b.Interface.VlanID == nil) {
		return false
	}
	if a.Interface.VlanID != nil && b.Interface.VlanID != nil &&
		*a.Interface.VlanID != *b.Interface.VlanID {
		return false
	}

	// Compare Feature if present
	if (a.Feature == nil) != (b.Feature == nil) {
		return false
	}
	if a.Feature != nil && b.Feature != nil {
		if a.Feature.EnableBindDhcpIP != b.Feature.EnableBindDhcpIP ||
			a.Feature.EnablePxe != b.Feature.EnablePxe ||
			a.Feature.EnableZtp != b.Feature.EnableZtp {
			return false
		}
	}

	return true
}
