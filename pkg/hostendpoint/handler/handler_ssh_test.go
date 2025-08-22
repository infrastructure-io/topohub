package handler

import (
	"context"
	"errors"
	"testing"

	gomonkey "github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/infrastructure-io/topohub/pkg/clients/kube"
	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/clients/ssh"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

func TestSSHHostEndpointHnadlerRefreshSession(t *testing.T) {
	ctx := context.TODO()
	obj := &topohubv1beta1.HostEndpoint{
		Spec: topohubv1beta1.HostEndpointSpec{
			IPAddr:          "192.168.1.1",
			Port:            ptr.To(int32(80)),
			HTTPS:           ptr.To(false),
			SecretName:      ptr.To("test-secret"),
			SecretNamespace: ptr.To("test-namespace"),
		},
	}
	auth := kube.AuthenticationSecret{
		Username: "admin",
		Password: "123456",
	}
	fakePool := &fakeSSHSesionPool{
		mockRefresh: func(_sessionID string, _cfg any) (pool.Session[ssh.Client], error) {
			return nil, nil
		},
	}

	globPatches := gomonkey.NewPatches()
	defer globPatches.Reset()

	globPatches.ApplyFuncReturn(kube.GetAuthenticationSecret,
		&auth, nil)

	t.Run("auth is nil and get secret failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFuncReturn(kube.GetAuthenticationSecret,
			nil, errors.New("test get auth secret error"))

		handler := &SSHHostEndpointHandler{
			sessionPool: fakePool,
		}
		err := handler.RefreshSession(ctx, obj, nil, log.Logger)
		require.ErrorContains(t, err, "test get auth secret error")
	})

	t.Run("auth not nil", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFuncReturn(kube.GetAuthenticationSecret,
			nil, errors.New("test get auth secret error"))

		handler := &SSHHostEndpointHandler{
			sessionPool: fakePool,
		}
		err := handler.RefreshSession(ctx, obj, &auth, log.Logger)
		require.NoError(t, err)
	})

	t.Run("refresh failed", func(t *testing.T) {
		handler := &SSHHostEndpointHandler{
			sessionPool: &fakeSSHSesionPool{
				mockRefresh: func(_sessionID string, _cfg any) (pool.Session[ssh.Client], error) {
					return nil, errors.New("test refresh session error")
				},
			},
		}
		err := handler.RefreshSession(ctx, obj, &auth, log.Logger)
		require.ErrorContains(t, err, "test refresh session error")
	})

	t.Run("refresh successful", func(t *testing.T) {
		handler := &SSHHostEndpointHandler{
			sessionPool: fakePool,
		}
		err := handler.RefreshSession(ctx, obj, nil, log.Logger)
		require.NoError(t, err)
	})
}

func TestSSHHostEndpointHnadlerCreateStatusIfNotExist(t *testing.T) {
	ctx := context.TODO()
	obj := topohubv1beta1.HostEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: "hostname",
		},
	}
	fakecli := fakeSSHStatusClient()
	handler := &SSHHostEndpointHandler{
		client: fakecli,
	}
	b, err := handler.CreateStatusIfNotExist(ctx, &obj, log.Logger)
	require.NoError(t, err)
	assert.True(t, b)

	b, err = handler.CreateStatusIfNotExist(ctx, &obj, log.Logger)
	require.NoError(t, err)
	assert.False(t, b)

	var status topohubv1beta1.SSHStatus
	err = fakecli.Get(ctx, client.ObjectKeyFromObject(&obj), &status)
	require.NoError(t, err)
	assert.Len(t, status.OwnerReferences, 1)
}

type fakeSSHSesionPool struct {
	mockRefresh func(sessionID string, cfg any) (pool.Session[ssh.Client], error)
}

func (fakeSSHSesionPool) GetOrCreate(sessionID string, cfg any) (pool.Session[ssh.Client], error) {
	return nil, nil
}

func (f *fakeSSHSesionPool) Refresh(sessionID string, cfg any) (pool.Session[ssh.Client], error) {
	return f.mockRefresh(sessionID, cfg)
}

func fakeSSHStatusClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(topohubv1beta1.SchemeGroupVersion,
		&topohubv1beta1.SSHStatus{},
		&topohubv1beta1.SSHStatusList{},
	)
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()
}
