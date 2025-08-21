package hostendpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	gomonkey "github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/clients/kube"
	"github.com/infrastructure-io/topohub/pkg/hostendpoint/handler"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	mockkube "github.com/infrastructure-io/topohub/pkg/mocks/kube"
)

func TestHostEndpointReconcilerReconcile(t *testing.T) {
	const name = "test-hostendpoint"

	ctx := context.TODO()
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: name,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var sshHandleCalled, redfishHandleCalled bool
	globPatches := gomonkey.NewPatches()
	defer globPatches.Reset()

	globPatches.ApplyMethodFunc((*handler.SSHHostEndpointHandler)(nil), "RefreshSession",
		func(_ctx context.Context,
			_hostEndpoint *topohubv1beta1.HostEndpoint,
			auth *kube.AuthenticationSecret,
			_logger *zap.SugaredLogger,
		) error {
			sshHandleCalled = true
			return nil
		})
	globPatches.ApplyMethodFunc((*handler.SSHHostEndpointHandler)(nil), "CreateStatusIfNotExist",
		func(_ctx context.Context,
			_hostEndpoint *topohubv1beta1.HostEndpoint,
			_logger *zap.SugaredLogger,
		) (bool, error) {
			sshHandleCalled = true
			return true, nil
		})
	globPatches.ApplyMethodFunc((*handler.RedfishHostEndpointHandler)(nil), "RefreshSession",
		func(_ctx context.Context,
			_hostEndpoint *topohubv1beta1.HostEndpoint,
			auth *kube.AuthenticationSecret,
			_logger *zap.SugaredLogger,
		) error {
			redfishHandleCalled = true
			return nil
		})
	globPatches.ApplyMethodFunc((*handler.RedfishHostEndpointHandler)(nil), "CreateStatusIfNotExist",
		func(_ctx context.Context,
			_hostEndpoint *topohubv1beta1.HostEndpoint,
			_logger *zap.SugaredLogger,
		) (bool, error) {
			redfishHandleCalled = true
			return true, nil
		})

	t.Run("not found", func(t *testing.T) {
		defer func() {
			redfishHandleCalled = false
			sshHandleCalled = false
		}()
		mgr := mockkube.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(fakeHostEndpointClient()).AnyTimes()
		mgr.EXPECT().GetCache().Return(nil).AnyTimes()

		reconciler := NewHostEndpointReconciler(mgr)
		res, err := reconciler.Reconcile(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)
	})

	t.Run("ssh endpoint", func(t *testing.T) {
		defer func() {
			redfishHandleCalled = false
			sshHandleCalled = false
		}()
		mgr := mockkube.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(fakeHostEndpointClient(&topohubv1beta1.HostEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: topohubv1beta1.HostEndpointSpec{
				Type: ptr.To(topohubv1beta1.EndpointTypeSSH),
			},
		})).AnyTimes()
		mgr.EXPECT().GetCache().Return(nil).AnyTimes()

		reconciler := NewHostEndpointReconciler(mgr)
		res, err := reconciler.Reconcile(ctx, req)
		require.NoError(t, err)
		assert.True(t, sshHandleCalled)
		assert.Equal(t, reconcile.Result{}, res)
	})

	t.Run("redfish endpoint", func(t *testing.T) {
		defer func() {
			redfishHandleCalled = false
			sshHandleCalled = false
		}()
		mgr := mockkube.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(fakeHostEndpointClient(&topohubv1beta1.HostEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: topohubv1beta1.HostEndpointSpec{
				Type: ptr.To(topohubv1beta1.EndpointTypeRedfish),
			},
		})).AnyTimes()
		mgr.EXPECT().GetCache().Return(nil).AnyTimes()

		reconciler := NewHostEndpointReconciler(mgr)
		res, err := reconciler.Reconcile(ctx, req)
		require.NoError(t, err)
		assert.True(t, redfishHandleCalled)
		assert.Equal(t, reconcile.Result{}, res)
	})

	t.Run("default endpoint type", func(t *testing.T) {
		defer func() {
			redfishHandleCalled = false
			sshHandleCalled = false
		}()
		mgr := mockkube.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(fakeHostEndpointClient(&topohubv1beta1.HostEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: topohubv1beta1.HostEndpointSpec{},
		})).AnyTimes()
		mgr.EXPECT().GetCache().Return(nil).AnyTimes()

		t.Run("refresh session failed and requeue when create status failed", func(t *testing.T) {
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			patches.ApplyMethodReturn((*handler.RedfishHostEndpointHandler)(nil), "RefreshSession",
				errors.New("test refresh session error"))
			patches.ApplyMethodReturn((*handler.RedfishHostEndpointHandler)(nil), "CreateStatusIfNotExist",
				false, errors.New("test create status error"))

			reconciler := NewHostEndpointReconciler(mgr)
			res, err := reconciler.Reconcile(ctx, req)
			require.NoError(t, err)
			assert.Equal(t, 2*time.Second, res.RequeueAfter)
		})

		t.Run("create status failed and no requeue when creating status conflicts", func(t *testing.T) {
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			gr := topohubv1beta1.SchemeGroupVersion.WithResource("redfishStatus").GroupResource()
			patches.ApplyMethodReturn((*handler.RedfishHostEndpointHandler)(nil), "CreateStatusIfNotExist",
				false, apierrors.NewConflict(gr, name, errors.New("test error")))

			reconciler := NewHostEndpointReconciler(mgr)
			res, err := reconciler.Reconcile(ctx, req)
			require.NoError(t, err)
			assert.Equal(t, reconcile.Result{}, res)
		})
	})
}

func fakeHostEndpointClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(topohubv1beta1.SchemeGroupVersion,
		&topohubv1beta1.HostEndpoint{},
		&topohubv1beta1.HostEndpointList{},
	)
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()
}
