package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// use fake client to test GetSubnetNameByIP
func TestGetSubnetNameByIP(t *testing.T) {
	// create logger for testing
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	// create scheme and register API types
	scheme := runtime.NewScheme()
	require.NoError(t, topohubv1beta1.AddToScheme(scheme))

	// test cases
	tests := []struct {
		name           string
		ip             string
		subnets        []topohubv1beta1.Subnet
		expectedSubnet string
		expectError    bool
	}{
		{
			name: "IP in subnet CIDR range",
			ip:   "192.168.1.100",
			subnets: []topohubv1beta1.Subnet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-subnet-1",
					},
					Spec: topohubv1beta1.SubnetSpec{
						IPv4Subnet: topohubv1beta1.IPv4SubnetSpec{
							Subnet: "192.168.1.0/24",
						},
					},
				},
			},
			expectedSubnet: "test-subnet-1",
			expectError:    false,
		},
		{
			name: "IP not in any subnet CIDR range",
			ip:   "10.0.0.1",
			subnets: []topohubv1beta1.Subnet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-subnet-2",
					},
					Spec: topohubv1beta1.SubnetSpec{
						IPv4Subnet: topohubv1beta1.IPv4SubnetSpec{
							Subnet: "192.168.1.0/24",
						},
					},
				},
			},
			expectedSubnet: "",
			expectError:    false,
		},
		{
			name: "multiple subnets, IP in one subnet range",
			ip:   "10.0.0.1",
			subnets: []topohubv1beta1.Subnet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-subnet-3",
					},
					Spec: topohubv1beta1.SubnetSpec{
						IPv4Subnet: topohubv1beta1.IPv4SubnetSpec{
							Subnet: "192.168.1.0/24",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-subnet-4",
					},
					Spec: topohubv1beta1.SubnetSpec{
						IPv4Subnet: topohubv1beta1.IPv4SubnetSpec{
							Subnet: "10.0.0.0/24",
						},
					},
				},
			},
			expectedSubnet: "test-subnet-4",
			expectError:    false,
		},
		{
			name: "invalid IP address",
			ip:   "invalid-ip",
			subnets: []topohubv1beta1.Subnet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-subnet-5",
					},
					Spec: topohubv1beta1.SubnetSpec{
						IPv4Subnet: topohubv1beta1.IPv4SubnetSpec{
							Subnet: "192.168.1.0/24",
						},
					},
				},
			},
			expectedSubnet: "",
			expectError:    true,
		},
		{
			name: "invalid CIDR in subnet",
			ip:   "192.168.1.100",
			subnets: []topohubv1beta1.Subnet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-subnet-6",
					},
					Spec: topohubv1beta1.SubnetSpec{
						IPv4Subnet: topohubv1beta1.IPv4SubnetSpec{
							Subnet: "invalid-cidr",
						},
					},
				},
			},
			expectedSubnet: "",
			expectError:    false,
		},
		{
			name: "empty CIDR in subnet",
			ip:   "192.168.1.100",
			subnets: []topohubv1beta1.Subnet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-subnet-7",
					},
					Spec: topohubv1beta1.SubnetSpec{
						IPv4Subnet: topohubv1beta1.IPv4SubnetSpec{
							Subnet: "",
						},
					},
				},
			},
			expectedSubnet: "",
			expectError:    false,
		},
	}

	// create a single fake client for all test cases
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// clear any existing subnets from previous tests
			existingSubnets := &topohubv1beta1.SubnetList{}
			require.NoError(t, fakeClient.List(context.Background(), existingSubnets))
			for _, subnet := range existingSubnets.Items {
				require.NoError(t, fakeClient.Delete(context.Background(), &subnet))
			}

			// add subnet resources to fake client
			for _, subnet := range tc.subnets {
				require.NoError(t, fakeClient.Create(context.Background(), &subnet))
			}

			// call the function to be tested
			subnet, err := GetSubnetNameByIP(tc.ip, fakeClient, sugar)

			// verify the result
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedSubnet, subnet)
			}
		})
	}
}
