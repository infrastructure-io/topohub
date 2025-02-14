package bindingip

import (
	"context"
	"fmt"
	"net"
	"sync"
	"reflect"
	bindingipdata "github.com/infrastructure-io/topohub/pkg/bindingip/data"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/tools"
	"github.com/infrastructure-io/topohub/pkg/config"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/infrastructure-io/topohub/pkg/log"
)

var bindingIPLock = &sync.Mutex{}

// bindingIPController 定义控制器结构
type bindingIPController struct {
	client   client.Client
	log      *zap.SugaredLogger
	config   *config.AgentConfig
}

// NewBindingIPController 创建新的控制器实例
func NewBindingIPController(mgr ctrl.Manager, config *config.AgentConfig) *bindingIPController {
	return &bindingIPController{
		client:   mgr.GetClient(),
		log:      log.Logger.Named("bindingipReconcile"),
		config:   config,
	}
}

// processBindingIP 处理单个 BindingIP 实例
func (c *bindingIPController) processBindingIP(bindingIP *topohubv1beta1.BindingIp, logger *zap.SugaredLogger) error {
	// 获取名称
	name := bindingIP.Name

	info := bindingipdata.BindingIPInfo{
		Subnet:  bindingIP.Spec.Subnet,
		IPAddr:  bindingIP.Spec.IpAddr,
		MacAddr: bindingIP.Spec.MacAddr,
		Valid:   bindingIP.Status.Valid,
	}

	// 更新本地缓存
	bindingIPLock.Lock()
	defer bindingIPLock.Unlock()

	if oldData := bindingipdata.BindingIPCacheDatabase.Get(name); oldData == nil {
		// 更新缓存
		bindingipdata.BindingIPCacheDatabase.Add(name, info)
		logger.Infof("new bindingIP added to cache: %+v", info)

	} else {
		if oldData.IPAddr != bindingIP.Spec.IpAddr || oldData.MacAddr != bindingIP.Spec.MacAddr || oldData.Subnet != bindingIP.Spec.Subnet {
			logger.Infof("bindingIP Spec changed, notify the dhcp server")
			bindingipdata.BindingIPCacheDatabase.Add(name, info)
		} else if !reflect.DeepEqual(oldData, info) {
			logger.Infof("bindingIP status changed")
			bindingipdata.BindingIPCacheDatabase.Add(name, info)
		} else {
			logger.Debugf("bindingIP does not change")
		}
	}

	return nil
}

// Reconcile 实现 reconcile.Reconciler 接口
func (c *bindingIPController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.With("bindingIP", req.NamespacedName)

	// 获取 BindingIP 实例
	bindingIP := &topohubv1beta1.BindingIp{}
	err := c.client.Get(ctx, req.NamespacedName, bindingIP)
	if err != nil {
		if errors.IsNotFound(err) {
			// 对象已被删除，从缓存中移除
			if bindingipdata.BindingIPCacheDatabase.Get(req.Name) != nil {
				bindingipdata.BindingIPCacheDatabase.Delete(req.Name)
			}
			return ctrl.Result{}, nil
		}
		logger.Errorf("failed to get BindingIP: %v", err)
		return ctrl.Result{}, err
	}

	// update the status of the BindingIP
	if err := c.updateBindingIPStatus(bindingIP, logger); err != nil {
		if errors.IsConflict(err) {
			logger.Debugf("conflict occurred while updating BindingIP status, will retry")
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Errorf("failed to update BindingIP status: %v", err)
		return ctrl.Result{}, fmt.Errorf("failed to update BindingIP status: %v", err)
	}

	// 处理 BindingIP 在 dhcp server 中的 绑定
	if err := c.processBindingIP(bindingIP, logger); err != nil {
		logger.Errorf("failed to process BindingIP: %v", err)
		return ctrl.Result{}, fmt.Errorf("failed to process BindingIP: %v", err)
	}

	return ctrl.Result{}, nil
}

// update the status of the BindingIP
func (c *bindingIPController) updateBindingIPStatus(bindingIP *topohubv1beta1.BindingIp, logger *zap.SugaredLogger) error {
	updated := bindingIP.DeepCopy()

	// 获取对应的 subnet 对象
	subnet := &topohubv1beta1.Subnet{}
	err := c.client.Get(context.TODO(), client.ObjectKey{Name: updated.Spec.Subnet}, subnet)
	if err != nil {
		updated.Status.Valid = false
		logger.Debugf("subnet %s not found, set status.Valid to false", updated.Spec.Subnet)
	} else {
		// 验证 IP 地址是否在子网范围内
		ip := net.ParseIP(updated.Spec.IpAddr)
		if ip != nil && tools.IsIPInRange(ip, subnet.Spec.IPv4Subnet.IPRange) {
			updated.Status.Valid = true
			logger.Debugf("IP %s is in subnet %s range %s, set status.Valid to true",
			updated.Spec.IpAddr, updated.Spec.Subnet, subnet.Spec.IPv4Subnet.IPRange)
		} else {
			updated.Status.Valid = false
			logger.Debugf("IP %s is not in subnet %s range %s, set status.Valid to false",
			updated.Spec.IpAddr, updated.Spec.Subnet, subnet.Spec.IPv4Subnet.IPRange)
		}
	}

	if !reflect.DeepEqual(updated.Status, bindingIP.Status) {
		logger.Debugf("status change to %+v, updating", updated.Status)

		// 使用 resource version 进行冲突检测更新
		updated.ResourceVersion = bindingIP.ResourceVersion
		if err := c.client.Status().Update(context.TODO(), updated); err != nil {
			logger.Errorf("failed to update status: %v", err)
			return err
		}
	}

	return nil
}

// SetupWithManager 设置控制器与管理器
func (c *bindingIPController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.BindingIp{}).
		Complete(c)
}
