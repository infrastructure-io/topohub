package hostoperation

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/agent/config"
	"github.com/infrastructure-io/topohub/pkg/agent/hoststatus/data"
	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/redfish"
	"go.uber.org/zap"
)

// HostOperationController reconciles a HostOperation object
type HostOperationController struct {
	client.Client
	Scheme      *runtime.Scheme
	agentConfig *config.AgentConfig
}

func NewHostOperationController(mgr ctrl.Manager, agentConfig *config.AgentConfig) (*HostOperationController, error) {
	return &HostOperationController{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		agentConfig: agentConfig,
	}, nil
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *HostOperationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.Logger.Named("HostOperationController").With(
		zap.String("HostOperation", req.Name),
	)

	logger.Debugf("Starting reconcile for HostOperation %s", req.Name)

	// 获取 HostOperation 对象
	hostOp := &bmcv1beta1.HostOperation{}
	if err := r.Get(ctx, req.NamespacedName, hostOp); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 获取关联的 HostStatus
	hostStatus := &bmcv1beta1.HostStatus{}
	if err := r.Get(ctx, client.ObjectKey{Name: hostOp.Spec.HostStatusName}, hostStatus); err != nil {
		logger.Errorf("Failed to get HostStatus %s: %v", hostOp.Spec.HostStatusName, err)
		return ctrl.Result{}, err
	}

	// 检查是否需要处理该对象
	if hostStatus.Status.ClusterAgent != r.agentConfig.ClusterAgentName {
		logger.Infof("Skipping HostOperation %s as it belongs to agent %s", hostOp.Name, hostStatus.Status.ClusterAgent)
		return ctrl.Result{}, nil
	}

	// 检查状态是否为空
	if hostOp.Status.Status == "" || hostOp.Status.Status == bmcv1beta1.HostOperationStatusPending {
		logger.Infof("Processing HostOperation %s : %+v", hostOp.Name, hostOp.Spec)

		// 更新状态
		hostOp.Status.Status = bmcv1beta1.HostOperationStatusPending
		hostOp.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		hostOp.Status.ClusterAgent = r.agentConfig.ClusterAgentName
		hostOp.Status.IpAddr = hostStatus.Status.Basic.IpAddr

		// 调用 redfish 接口 完成操作
		// get connect config from cache
		d := data.HostCacheDatabase.Get(hostOp.Spec.HostStatusName)
		if d == nil {
			hostOp.Status.Status = bmcv1beta1.HostOperationStatusPending
			logger.Warnf("Failed to get connect config %s from cache, retry later", hostOp.Spec.HostStatusName)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
		logger.Debugf("get connect config %s from cache: %+v", hostOp.Spec.HostStatusName, d)

		var err error
		c, terr := redfish.NewClient(*d, logger)
		if terr != nil {
			err = terr
			logger.Errorf("Failed to operate %s: %v", hostOp.Spec.HostStatusName, err)
			hostOp.Status.Status = bmcv1beta1.HostOperationStatusFailed
			hostOp.Status.Message = err.Error()
		} else {
			switch hostOp.Spec.Action {
			case bmcv1beta1.BootCmdOn:
				err = c.Power(hostOp.Spec.Action)
			case bmcv1beta1.BootCmdForceOn:
				err = c.Power(hostOp.Spec.Action)
			case bmcv1beta1.BootCmdForceOff:
				err = c.Power(hostOp.Spec.Action)
			case bmcv1beta1.BootCmdGracefulShutdown:
				err = c.Power(hostOp.Spec.Action)
			case bmcv1beta1.BootCmdForceRestart:
				err = c.Power(hostOp.Spec.Action)
			case bmcv1beta1.BootCmdGracefulRestart:
				err = c.Power(hostOp.Spec.Action)
			case bmcv1beta1.BootCmdResetPxeOnce:
				err = c.Power(hostOp.Spec.Action)
			default:
				err = fmt.Errorf("invalid action %s", hostOp.Spec.Action)
			}
		}

		hostOp.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			logger.Errorf("Failed to operate %s: %v", hostOp.Spec.HostStatusName, err)
			hostOp.Status.Status = bmcv1beta1.HostOperationStatusFailed
			hostOp.Status.Message = err.Error()
		} else {
			logger.Infof("Succeeded to operate %s", hostOp.Spec.HostStatusName)
			hostOp.Status.Status = bmcv1beta1.HostOperationStatusSuccess
		}

		// 更新
		if err := r.Status().Update(ctx, hostOp); err != nil {
			logger.Errorf("Action has been done, but failed to update HostOperation status: %v", err)
			return ctrl.Result{}, fmt.Errorf("failed to update HostOperation status: %v", err)
		}
		logger.Debugf("Successfully updated HostOperation %s status", hostOp.Name)

	} else {
		logger.Infof("HostOperation %s has been processed", hostOp.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *HostOperationController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bmcv1beta1.HostOperation{}).
		Complete(r)
}
