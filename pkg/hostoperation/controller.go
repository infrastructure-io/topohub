package hostoperation

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	logpkg "github.com/infrastructure-io/topohub/pkg/log"
)

// HostOperationController reconciles a HostOperation object
type HostOperationController struct {
	Scheme      *runtime.Scheme
	agentConfig *config.AgentConfig
	redfishPool pool.SessionPool[redfish.Client]
	log         *zap.SugaredLogger

	client.Client
}

func NewHostOperationController(mgr ctrl.Manager, agentConfig *config.AgentConfig) (*HostOperationController, error) {
	return &HostOperationController{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		agentConfig: agentConfig,
		redfishPool: redfish.GetSessionPool(),
		log:         logpkg.Logger.Named("HostOperationController"),
	}, nil
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *HostOperationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.log.With("hostoperation", req.Name)
	logger.Debug("Starting reconcile for HostOperation")

	// get the HostOperation object
	var hostOp topohubv1beta1.HostOperation
	if err := r.Get(ctx, req.NamespacedName, &hostOp); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// get the RedfishStatus
	redfishStatus := &topohubv1beta1.RedfishStatus{}
	if err := r.Get(ctx, client.ObjectKey{Name: hostOp.Spec.RedfishStatusName}, redfishStatus); err != nil {
		logger.Errorf("Failed to get RedfishStatus %s: %v", hostOp.Spec.RedfishStatusName, err)
		return ctrl.Result{}, err
	}

	// check if the host operation is pending
	if hostOp.Status.Status == "" || hostOp.Status.Status == topohubv1beta1.HostOperationStatusPending {
		logger.Infof("Processing HostOperation %s : %+v", hostOp.Name, hostOp.Spec)

		// update status
		hostOp.Status.Status = topohubv1beta1.HostOperationStatusPending
		hostOp.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)

		// get HostEndpoint
		hostEndpoint, connErr := r.getHostEndpoint(redfishStatus)
		if connErr != nil {
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusPending
			logger.Warnf("Failed to get connection info for %s: %v, retry later", hostOp.Spec.RedfishStatusName, connErr)
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
		logger.Debugf("get connect config %s: %+v", hostOp.Spec.RedfishStatusName, hostEndpoint)
		hostOp.Status.ClusterName = *hostEndpoint.Spec.ClusterName
		hostOp.Status.IpAddr = hostEndpoint.Spec.IPAddr

		// get secret data
		username, password, err := r.getSecretData(
			*hostEndpoint.Spec.SecretName,
			*hostEndpoint.Spec.SecretNamespace,
		)
		if err != nil {
			r.log.Errorf("Failed to get secret data for HostEndpoint %s: %v", hostEndpoint.Name, err)
			return ctrl.Result{}, err
		}

		sessionCfg := &redfish.RedfishSessionConfig{
			Username: username,
			Password: password,
			IPAddr:   hostEndpoint.Spec.IPAddr,
			Port:     int(*hostEndpoint.Spec.Port),
			Https:    *hostEndpoint.Spec.HTTPS,
		}
		session, err := r.redfishPool.GetOrCreate(sessionCfg.SessionID(), sessionCfg)
		if err != nil {
			logger.Errorf("Failed to operate %s: %v", hostOp.Spec.RedfishStatusName, err)
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
			hostOp.Status.Message = err.Error()
		} else {
			c := session.GetClient()
			switch hostOp.Spec.Action {
			case topohubv1beta1.BootCmdOn:
				err = c.Power(hostOp.Spec.Action)
			case topohubv1beta1.BootCmdForceOn:
				err = c.Power(hostOp.Spec.Action)
			case topohubv1beta1.BootCmdForceOff:
				err = c.Power(hostOp.Spec.Action)
			case topohubv1beta1.BootCmdGracefulShutdown:
				err = c.Power(hostOp.Spec.Action)
			case topohubv1beta1.BootCmdForceRestart:
				err = c.Power(hostOp.Spec.Action)
			case topohubv1beta1.BootCmdGracefulRestart:
				err = c.Power(hostOp.Spec.Action)
			case topohubv1beta1.BootCmdResetPxeOnce:
				// check pxe boot method
				if strings.ToLower(hostEndpoint.Spec.PxeBootType) == topohubv1beta1.PxeBootTypeIPMI {
					// use IPMI command to set PXE boot
					logger.Infof("Using IPMI to set PXE boot for %s", hostEndpoint.Spec.IPAddr)
					err = r.performIPMIPXEBoot(sessionCfg.IPAddr, sessionCfg.Username, sessionCfg.Password)
					if err == nil {
						hostOp.Status.Message = fmt.Sprintf("Successfully performed PXE boot via IPMI for %s", hostEndpoint.Spec.IPAddr)
					}
				} else {
					// use default Redfish client
					logger.Infof("Using Redfish to set PXE boot for %s", hostEndpoint.Spec.IPAddr)
					err = c.Power(hostOp.Spec.Action)
					if err == nil {
						hostOp.Status.Message = fmt.Sprintf("Successfully performed PXE boot via Redfish method for %s", hostEndpoint.Spec.IPAddr)
						logger.Info(hostOp.Status.Message)
					}
				}
			default:
				err = fmt.Errorf("invalid action %s", hostOp.Spec.Action)
			}
		}

		hostOp.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			logger.Errorf("Failed to operate %s: %v", hostOp.Spec.RedfishStatusName, err)
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusFailed
			hostOp.Status.Message = err.Error()
		} else {
			logger.Infof("Succeeded to operate %s", hostOp.Spec.RedfishStatusName)
			hostOp.Status.Status = topohubv1beta1.HostOperationStatusSuccess
		}

		// 更新
		if err := r.Status().Update(ctx, &hostOp); err != nil {
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
		For(&topohubv1beta1.HostOperation{}).
		Complete(r)
}

func (r *HostOperationController) getHostEndpoint(redfishStatus *topohubv1beta1.RedfishStatus) (*topohubv1beta1.HostEndpoint, error) {
	// all RedfishStatus should have ownerReferences
	if len(redfishStatus.OwnerReferences) > 0 {
		for _, ownerRef := range redfishStatus.OwnerReferences {
			if ownerRef.Kind == topohubv1beta1.KindHostEndpoint {
				r.log.Infof("Found HostEndpoint owner reference: %s", ownerRef.Name)
				// get HostEndpoint
				hostEndpoint := &topohubv1beta1.HostEndpoint{}
				if err := r.Get(context.TODO(), client.ObjectKey{Name: ownerRef.Name}, hostEndpoint); err != nil {
					r.log.Errorf("Failed to get HostEndpoint %s: %v", ownerRef.Name, err)
					return nil, err
				}
				return hostEndpoint, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to get HostEndpoint for RedfishStatus %s", redfishStatus.Name)
}

// getSecretData 从 Secret 中获取用户名和密码
func (r *HostOperationController) getSecretData(secretName, secretNamespace string) (string, string, error) {
	r.log.Debugf("Attempting to get secret data for %s/%s", secretNamespace, secretName)

	// 使用 controller-runtime client 获取 Secret
	var secret corev1.Secret
	objKey := client.ObjectKey{Name: secretName, Namespace: secretNamespace}
	if err := r.Get(context.TODO(), objKey, &secret); err != nil {
		r.log.Errorf("Failed to get secret %s/%s: %v", secretNamespace, secretName, err)
		return "", "", err
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	r.log.Debugf("Successfully retrieved secret data for %s/%s", secretNamespace, secretName)
	return username, password, nil
}

// performIPMIPXEBoot perform IPMI PXE boot
func (r *HostOperationController) performIPMIPXEBoot(ipAddr, username, password string) error {
	// set PXE boot via IPMI
	r.log.Infof("Setting PXE boot via IPMI for %s", ipAddr)

	// build set pxe boot command
	setPxeCmd := exec.Command("ipmitool", "-I", "lanplus", "-H", ipAddr, "-U", username, "-P", password, "chassis", "bootdev", "pxe")
	// print command (hide password)
	r.log.Infof("Executing command: ipmitool -I lanplus -H %s -U %s -P %s chassis bootdev pxe", ipAddr, username, password)

	// execute command
	output, err := setPxeCmd.CombinedOutput()
	if err != nil {
		r.log.Errorf("Failed to set PXE boot via IPMI: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to set PXE boot via IPMI: %v", err)
	}
	r.log.Infof("Successfully set PXE boot via IPMI for %s: %s", ipAddr, strings.TrimSpace(string(output)))

	// perform power reset via IPMI
	r.log.Infof("Performing power reset via IPMI for %s", ipAddr)

	// build power reset command
	resetCmd := exec.Command("ipmitool", "-I", "lanplus", "-H", ipAddr, "-U", username, "-P", password, "power", "reset")
	// print command (hide password)
	r.log.Infof("Executing command: ipmitool -I lanplus -H %s -U %s -P %s power reset", ipAddr, username, password)

	// execute command
	output, err = resetCmd.CombinedOutput()
	if err != nil {
		r.log.Errorf("Failed to perform power reset via IPMI: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to perform power reset via IPMI: %v", err)
	}
	r.log.Infof("Successfully performed power reset via IPMI for %s: %s", ipAddr, strings.TrimSpace(string(output)))

	return nil
}
