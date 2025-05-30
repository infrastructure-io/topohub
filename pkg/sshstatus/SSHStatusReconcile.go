// 完成对 sshstatus 的 SSH 信息更新

package sshstatus

import (
	"context"
	"fmt"
	"time"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	sshstatusdata "github.com/infrastructure-io/topohub/pkg/sshstatus/data"

	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/sshstatus/ssh"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ------------------------------  update the status.info of the sshstatus

// UpdateSSHStatusInfo 更新SSH状态信息
func (c *sshStatusController) UpdateSSHStatusInfo(name string, d *sshstatusdata.SSHConnectCon) (bool, error) {
	// 获取锁以更新SSH状态实例
	c.log.Debugf("lock for updating sshStatus instance %s", name)
	lock := lock.LockManagerInstance.GetLock(name)
	lock.Lock()
	defer lock.Unlock()

	// 创建SSH客户端
	var healthy bool
	client, err1 := ssh.NewClient(*d, c.log)
	if err1 != nil {
		c.log.Warnf("Failed to create SSH client for SSHStatus %s: %v", name, err1)
		healthy = false
	} else {
		defer client.Close()
		healthy = client.IsHealthy()
	}

	auth := "without username and password"
	if len(d.Username) != 0 && len(d.Password) != 0 {
		auth = "with username and password"
	} else if d.SSHKeyAuth && len(d.SSHKey) != 0 {
		auth = "with SSH key authentication"
	}
	c.log.Debugf("try to check SSH with url: %s:%d, %s", d.Info.IpAddr, d.Info.Port, auth)

	// 获取现有的SSHStatus
	existing := &topohubv1beta1.SSHStatus{}
	err := c.client.Get(context.Background(), types.NamespacedName{Name: name}, existing)
	if err != nil {
		c.log.Errorf("Failed to get SSHStatus %s: %v", name, err)
		return false, err
	}
	updated := existing.DeepCopy()

	// 检查健康状态
	updated.Status.Healthy = healthy
	updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)

	// 如果健康，获取系统信息
	if healthy {
		infoData, err := client.GetSystemInfo()
		if err != nil {
			c.log.Errorf("Failed to get info of SSHStatus %s: %v", name, err)
			healthy = false
		} else {
			updated.Status.Info = infoData
		}
	}

	// 如果状态没有变化，不更新
	if compareSSHStatus(updated.Status, existing.Status, c.log) {
		c.log.Debugf("SSHStatus %s has no changes, skipping update", name)
		return healthy, nil
	}

	// 更新状态
	c.log.Debugf("Updating SSHStatus %s", name)
	if err := c.client.Status().Update(context.Background(), updated); err != nil {
		if errors.IsConflict(err) {
			c.log.Debugf("Conflict updating SSHStatus %s, will retry", name)
			return healthy, err
		}
		c.log.Errorf("Failed to update SSHStatus %s: %v", name, err)
		return healthy, err
	}

	c.log.Infof("Successfully updated SSHStatus %s", name)
	return healthy, nil
}

// UpdateSSHStatusInfoWrapper 更新指定名称的SSH状态信息或所有SSH状态信息
func (c *sshStatusController) UpdateSSHStatusInfoWrapper(name string) error {
	if name != "" {
		// 更新指定的SSH状态
		d := sshstatusdata.SSHCacheDatabase.Get(name)
		if d == nil {
			c.log.Warnf("SSHStatus %s not found in cache", name)
			return fmt.Errorf("SSHStatus %s not found in cache", name)
		}

		_, err := c.UpdateSSHStatusInfo(name, d)
		if err != nil {
			c.log.Errorf("Failed to update SSHStatus %s: %v", name, err)
			return err
		}
	} else {
		// 更新所有SSH状态
		hosts := sshstatusdata.SSHCacheDatabase.GetAll()
		c.log.Debugf("Updating all %d SSH statuses", len(hosts))

		for name, d := range hosts {
			_, err := c.UpdateSSHStatusInfo(name, &d)
			if err != nil {
				c.log.Errorf("Failed to update SSHStatus %s: %v", name, err)
			}
		}
	}
	return nil
}

// UpdateSSHStatusAtInterval 定期更新所有SSH状态信息
func (c *sshStatusController) UpdateSSHStatusAtInterval() {
	interval := time.Duration(c.config.SSHStatusUpdateInterval) * time.Second
	if interval == 0 {
		interval = 60 * time.Second // 默认60秒
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	c.log.Infof("begin to update all sshStatus at interval of %v seconds", interval/time.Second)

	for {
		select {
		case <-c.stopCh:
			c.log.Info("Stopping UpdateSSHStatusAtInterval")
			return
		case <-ticker.C:
			c.log.Debugf("update all sshStatus at interval")
			if err := c.UpdateSSHStatusInfoWrapper(""); err != nil {
				c.log.Errorf("Failed to update SSH status: %v", err)
			}
		}
	}
}

// ------------------------------  sshstatus 的 reconcile, 触发更新

// processSSHStatus 处理SSH状态，缓存数据并更新状态信息
func (c *sshStatusController) processSSHStatus(sshStatus *topohubv1beta1.SSHStatus, logger *zap.SugaredLogger) error {
	logger.Debugf("Processing Existed SSHStatus: %s (Type: %s, IP: %s, Health: %v)",
		sshStatus.Name,
		sshStatus.Status.Basic.Type,
		sshStatus.Status.Basic.IpAddr,
		sshStatus.Status.Healthy)

	// 检查IP是否在Subnet的dhcpClientDetails中，并更新subnetName
	if err := c.updateSubnetNameFromDhcpClientDetails(sshStatus, logger); err != nil {
		logger.Warnf("Failed to update subnet name from dhcp client details: %v", err)
	}

	// 缓存SSH状态数据到本地
	username := ""
	password := ""
	sshKey := ""
	sshKeyAuth := false
	var err error
	if len(sshStatus.Status.Basic.SecretName) > 0 && len(sshStatus.Status.Basic.SecretNamespace) > 0 {
		username, password, sshKey, sshKeyAuth, err = c.getSecretData(
			sshStatus.Status.Basic.SecretName,
			sshStatus.Status.Basic.SecretNamespace,
		)
		if err != nil {
			logger.Errorf("Failed to get secret data for SSHStatus %s: %v", sshStatus.Name, err)
			return err
		}
		logger.Debugf("Adding/Updating SSHStatus %s in cache with username: %s, sshKeyAuth: %v",
			sshStatus.Name, username, sshKeyAuth)
	} else {
		logger.Debugf("Adding/Updating SSHStatus %s in cache with empty authentication", sshStatus.Name)
	}

	sshstatusdata.SSHCacheDatabase.Add(sshStatus.Name, sshstatusdata.SSHConnectCon{
		Info:       &sshStatus.Status.Basic,
		Username:   username,
		Password:   password,
		SSHKey:     sshKey,
		SSHKeyAuth: sshKeyAuth,
	})

	// 如果状态信息为空，进行首次更新
	if len(sshStatus.Status.Info) == 0 {
		if err := c.UpdateSSHStatusInfoWrapper(sshStatus.Name); err != nil {
			return err
		}
	} else {
		logger.Debugf("SSHStatus %s has already been processed, skipping the first time update", sshStatus.Name)
	}

	logger.Debugf("Successfully processed SSHStatus %s", sshStatus.Name)
	return nil
}

// Reconcile 实现 reconcile.Reconciler 接口
// 负责在 sshstatus 创建后 SSH 信息的第一次更新（后续的更新由 UpdateSSHStatusAtInterval 完成）
func (c *sshStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.With("sshstatus", req.Name)

	logger.Debugf("Reconciling SSHStatus %s", req.Name)

	// 获取 SSHStatus
	sshStatus := &topohubv1beta1.SSHStatus{}
	if err := c.client.Get(ctx, req.NamespacedName, sshStatus); err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("SSHStatus not found")
			data := sshstatusdata.SSHCacheDatabase.Get(req.Name)
			if data != nil {
				logger.Infof("delete sshStatus %s in cache, %+v", req.Name, *data)
				sshstatusdata.SSHCacheDatabase.Delete(req.Name)
			}
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SSHStatus")
		return ctrl.Result{}, err
	}

	// 检查是否刚创建的状态
	if len(sshStatus.Status.Basic.IpAddr) == 0 {
		c.log.Debugf("ignore sshStatus %s just created", sshStatus.Name)
		return ctrl.Result{}, nil
	}

	// 处理 SSHStatus
	if err := c.processSSHStatus(sshStatus, logger); err != nil {
		logger.Error(err, "Failed to process SSHStatus, will retry")
		return ctrl.Result{
			RequeueAfter: time.Second * 2,
		}, nil
	}

	logger.Debugf("Successfully processed SSHStatus %s", sshStatus.Name)
	return ctrl.Result{}, nil
}
