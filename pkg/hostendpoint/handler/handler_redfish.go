package handler

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/clients/kube"
	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

type RedfishHostEndpointHandler struct {
	client      client.Client
	cacheReader client.Reader
	sessionPool pool.SessionPool[redfish.Client]
}

// Check RedfishHostEndpointHandler implements the HostEndpointHandler
var _ HostEndpointHandler = (*RedfishHostEndpointHandler)(nil)

func (h *RedfishHostEndpointHandler) RefreshSession(
	ctx context.Context,
	hostEndpoint *topohubv1beta1.HostEndpoint,
	auth *kube.AuthenticationSecret,
	logger *zap.SugaredLogger,
) error {
	if auth == nil {
		var err error
		auth, err = kube.GetAuthenticationSecret(ctx, h.cacheReader,
			*hostEndpoint.Spec.SecretName, *hostEndpoint.Spec.SecretNamespace)
		if err != nil {
			return err
		}
	}
	sessionCfg := &redfish.RedfishSessionConfig{
		Username: auth.Username,
		Password: auth.Password,
		IPAddr:   hostEndpoint.Spec.IPAddr,
		Port:     int(*hostEndpoint.Spec.Port),
		Https:    *hostEndpoint.Spec.HTTPS,
	}
	if _, err := h.sessionPool.Refresh(sessionCfg.SessionID(), sessionCfg); err != nil && err != pool.ErrSessionNotFound {
		return err
	}
	return nil
}

func (h *RedfishHostEndpointHandler) CreateStatusIfNotExist(
	ctx context.Context,
	hostEndpoint *topohubv1beta1.HostEndpoint,
	logger *zap.SugaredLogger,
) (bool, error) {
	name := hostEndpoint.Name
	var existingStatus topohubv1beta1.RedfishStatus
	err := h.client.Get(ctx, client.ObjectKey{Name: name}, &existingStatus)
	if err == nil {
		logger.Infof("RedfishStatus %s already exists, no need to create", name)
		return false, nil
	}
	// if error is not not found, return error
	if !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to get RedfishStatus %s: %v", name, err)
	}

	// RedfishStatus doesn't exist, create new one
	logger.Debugf("Creating new RedfishStatus %s", name)
	if err := h.client.Create(ctx, &topohubv1beta1.RedfishStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelClientMode: topohubv1beta1.Redfish,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         topohubv1beta1.APIVersion,
					Kind:               topohubv1beta1.KindHostEndpoint,
					Name:               hostEndpoint.Name,
					UID:                hostEndpoint.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
	}); err != nil {
		return false, err
	}
	return true, nil
}
