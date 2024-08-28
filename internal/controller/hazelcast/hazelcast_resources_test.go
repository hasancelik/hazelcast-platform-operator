package hazelcast

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"testing"

	proto "github.com/hazelcast/hazelcast-go-client"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	"github.com/hazelcast/hazelcast-platform-operator/internal/faketest"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	hztypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func Test_configBaseDirShouldNotChangeWhenExists(t *testing.T) {
	RegisterFailHandler(faketest.Fail(t))
	nn := types.NamespacedName{Name: "hazelcast", Namespace: "default"}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: hazelcastv1alpha1.HazelcastSpec{
			ClusterSize: ptr.To(int32(3)),
			Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
				},
				Restore: hazelcastv1alpha1.RestoreConfiguration{
					BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
						BucketURI:  "s3://mybucketuri/dir",
						SecretName: "mysecretname",
					},
				},
			},
		},
	}
	basicConfig := config.HazelcastBasicConfig(h)
	basicConfig.Persistence.BaseDir = n.PersistenceMountPath
	basicConfig.Persistence.BackupDir = path.Join(n.PersistenceMountPath, "hot-backup")
	cfg, err := yaml.Marshal(config.HazelcastWrapper{Hazelcast: basicConfig})
	if err != nil {
		t.Errorf("Error forming config")
	}
	cm := &corev1.Secret{
		ObjectMeta: metadata(h),
		Data:       make(map[string][]byte),
	}
	mtlsSec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.MTLSCertSecretName,
			Namespace: nn.Namespace,
		},
	}
	cm.Data["hazelcast.yaml"] = cfg

	c := faketest.FakeK8sClient(h, cm, mtlsSec)

	hr := &faketest.FakeHttpClientRegistry{}
	hr.Set(nn.Namespace, &http.Client{})
	r := NewHazelcastReconciler(
		c,
		ctrl.Log.WithName("test").WithName("Hazelcast"),
		c.Scheme(),
		nil,
		&FakeHzClientRegistry{},
		&FakeHzStatusServiceRegistry{},
		hr,
	)
	_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: nn})
	if err != nil {
		t.Error(fmt.Errorf("unexpected reconcile error: %w", err))
	}
	actualConf, err := getHazelcastConfig(context.Background(), c, nn)
	if err != nil {
		t.Errorf("Error getting config")
	}
	Expect(actualConf.Hazelcast.Persistence.BaseDir).Should(Equal(n.PersistenceMountPath))
	Expect(actualConf.Hazelcast.Persistence.BackupDir).Should(Equal(path.Join(n.PersistenceMountPath, "hot-backup")))
}

func Test_EnsureClusterActive_ShouldChangeState(t *testing.T) {
	RegisterFailHandler(faketest.Fail(t))
	r := HazelcastReconciler{}
	c := &faketest.FakeHzClient{}
	clusterState := hztypes.ClusterStateNoMigration
	c.TInvokeOnRandomTarget = func(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
		if req.Type() == codec.MCGetClusterMetadataCodecRequestMessageType {
			return codec.EncodeMCGetClusterMetadataResponse(hztypes.ClusterMetadata{
				CurrentState: clusterState,
			}), nil
		}
		if req.Type() == codec.MCChangeClusterStateCodecRequestMessageType {
			clusterState = codec.DecodeMCChangeClusterStateRequest(req)
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected message type")
	}
	h := &hazelcastv1alpha1.Hazelcast{
		Spec: hazelcastv1alpha1.HazelcastSpec{
			Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				Restore: hazelcastv1alpha1.RestoreConfiguration{
					HotBackupResourceName: "my-hb-name",
				},
			},
		},
		Status: hazelcastv1alpha1.HazelcastStatus{
			Restore: hazelcastv1alpha1.RestoreStatus{
				State: hazelcastv1alpha1.RestoreSucceeded,
			},
			Phase: hazelcastv1alpha1.Running,
		},
	}
	Expect(r.ensureClusterActive(context.TODO(), c, h)).Should(Succeed())
	Expect(clusterState).To(Equal(hztypes.ClusterStateActive))
}
