package hostoperation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

func TestHostOperationWebhook_ValidateCreate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	// create scheme
	scheme := runtime.NewScheme()
	_ = topohubv1beta1.AddToScheme(scheme)

	// create fake client
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	// create webhook instance
	webhook := &HostOperationWebhook{
		Client: client,
		log:    sugar,
	}

	// define test cases
	tests := []struct {
		name        string
		hostOp      *topohubv1beta1.HostOperation
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid redfish operation",
			hostOp: &topohubv1beta1.HostOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-redfish-op",
				},
				Spec: topohubv1beta1.HostOperationSpec{
					HostType:   "Redfish",
					Action:     topohubv1beta1.RedfishCmdOn,
					StatusName: "test-status",
				},
			},
			expectError: false,
		},
		{
			name: "valid ssh operation",
			hostOp: &topohubv1beta1.HostOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ssh-op",
				},
				Spec: topohubv1beta1.HostOperationSpec{
					HostType:   "SSH",
					Action:     topohubv1beta1.SSHCmdRestart,
					StatusName: "test-status",
				},
			},
			expectError: false,
		},
		{
			name: "invalid operation type",
			hostOp: &topohubv1beta1.HostOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-invalid-type",
				},
				Spec: topohubv1beta1.HostOperationSpec{
					HostType:   "Invalid",
					Action:     topohubv1beta1.RedfishCmdOn,
					StatusName: "test-status",
				},
			},
			expectError: true,
			errorMsg:    "invalid type Invalid, must be either 'Redfish' or 'SSH'",
		},
		{
			name: "invalid redfish action",
			hostOp: &topohubv1beta1.HostOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-invalid-redfish-action",
				},
				Spec: topohubv1beta1.HostOperationSpec{
					HostType:   "Redfish",
					Action:     "InvalidAction",
					StatusName: "test-status",
				},
			},
			expectError: true,
			errorMsg:    "invalid action InvalidAction for Redfish operation type",
		},
		{
			name: "invalid ssh action",
			hostOp: &topohubv1beta1.HostOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-invalid-ssh-action",
				},
				Spec: topohubv1beta1.HostOperationSpec{
					HostType:   "SSH",
					Action:     "InvalidAction",
					StatusName: "test-status",
				},
			},
			expectError: true,
			errorMsg:    "invalid action InvalidAction for SSH operation type",
		},
	}

	// run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// for wrong object type test, handle it separately
			if tt.name == "wrong object type" {
				// create a non-HostOperation type object, but implements runtime.Object interface
				wrongObj := &topohubv1beta1.HostEndpoint{}
				_, err := webhook.ValidateCreate(context.Background(), wrongObj)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "expected a HostOperation")
				return
			}

			// normal test
			_, err := webhook.ValidateCreate(context.Background(), tt.hostOp)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
