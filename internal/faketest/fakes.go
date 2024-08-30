package faketest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"

	proto "github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func FakeK8sClient(initObjs ...client.Object) client.Client {
	scheme, _ := hazelcastv1alpha1.SchemeBuilder.
		Register(
			&hazelcastv1alpha1.Hazelcast{},
			&hazelcastv1alpha1.HazelcastList{},
			&hazelcastv1alpha1.Map{},
			&hazelcastv1alpha1.MapList{},
			&hazelcastv1alpha1.Cache{},
			&hazelcastv1alpha1.CacheList{},
			&hazelcastv1alpha1.HotBackup{},
			&hazelcastv1alpha1.HotBackupList{},
			&hazelcastv1alpha1.CronHotBackup{},
			&hazelcastv1alpha1.CronHotBackupList{},
			&hazelcastv1alpha1.MultiMap{},
			&hazelcastv1alpha1.MultiMapList{},
			&hazelcastv1alpha1.ReplicatedMap{},
			&hazelcastv1alpha1.ReplicatedMapList{},
			&hazelcastv1alpha1.Topic{},
			&hazelcastv1alpha1.TopicList{},
			&hazelcastv1alpha1.Queue{},
			&hazelcastv1alpha1.QueueList{},
			&hazelcastv1alpha1.JetJob{},
			&hazelcastv1alpha1.JetJobList{},
			&hazelcastv1alpha1.JetJobSnapshot{},
			&hazelcastv1alpha1.JetJobSnapshotList{},
			&hazelcastv1alpha1.WanReplication{},
			&hazelcastv1alpha1.WanReplicationList{},
			&hazelcastv1alpha1.WanSync{},
			&hazelcastv1alpha1.WanSyncList{},
			&hazelcastv1alpha1.HazelcastEndpoint{},
			&hazelcastv1alpha1.HazelcastEndpointList{},
			&hazelcastv1alpha1.UserCodeNamespace{},
			&hazelcastv1alpha1.VectorCollection{},
			&hazelcastv1alpha1.VectorCollectionList{}).
		Build()

	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjs...).
		WithStatusSubresource( // https://github.com/kubernetes-sigs/controller-runtime/issues/2362
			&hazelcastv1alpha1.Hazelcast{},
			&hazelcastv1alpha1.Map{},
			&hazelcastv1alpha1.Cache{},
			&hazelcastv1alpha1.HotBackup{},
			&hazelcastv1alpha1.CronHotBackup{},
			&hazelcastv1alpha1.MultiMap{},
			&hazelcastv1alpha1.ReplicatedMap{},
			&hazelcastv1alpha1.Topic{},
			&hazelcastv1alpha1.Queue{},
			&hazelcastv1alpha1.JetJob{},
			&hazelcastv1alpha1.JetJobSnapshot{},
			&hazelcastv1alpha1.WanReplication{},
			&hazelcastv1alpha1.WanSync{},
			&hazelcastv1alpha1.HazelcastEndpoint{},
			&hazelcastv1alpha1.UserCodeNamespace{},
			&hazelcastv1alpha1.VectorCollection{}).
		WithIndex(&hazelcastv1alpha1.Map{}, "hazelcastResourceName", func(o client.Object) []string {
			hzMap := o.(*hazelcastv1alpha1.Map)
			return []string{hzMap.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.Cache{}, "hazelcastResourceName", func(o client.Object) []string {
			cache := o.(*hazelcastv1alpha1.Cache)
			return []string{cache.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.HotBackup{}, "hazelcastResourceName", func(o client.Object) []string {
			backup := o.(*hazelcastv1alpha1.HotBackup)
			return []string{backup.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.CronHotBackup{}, "hazelcastResourceName", func(o client.Object) []string {
			cronBackup := o.(*hazelcastv1alpha1.CronHotBackup)
			return []string{cronBackup.Spec.HotBackupTemplate.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.MultiMap{}, "hazelcastResourceName", func(o client.Object) []string {
			mmap := o.(*hazelcastv1alpha1.MultiMap)
			return []string{mmap.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.ReplicatedMap{}, "hazelcastResourceName", func(o client.Object) []string {
			rmap := o.(*hazelcastv1alpha1.ReplicatedMap)
			return []string{rmap.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.Topic{}, "hazelcastResourceName", func(o client.Object) []string {
			topic := o.(*hazelcastv1alpha1.Topic)
			return []string{topic.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.Queue{}, "hazelcastResourceName", func(o client.Object) []string {
			queue := o.(*hazelcastv1alpha1.Queue)
			return []string{queue.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.JetJob{}, "hazelcastResourceName", func(o client.Object) []string {
			jj := o.(*hazelcastv1alpha1.JetJob)
			return []string{jj.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.WanReplication{}, "hazelcastResourceName", func(o client.Object) []string {
			wr := o.(*hazelcastv1alpha1.WanReplication)
			var hzResources []string
			for k := range wr.Status.WanReplicationMapsStatus {
				hzName, _ := splitWanMapKey(k)
				hzResources = append(hzResources, hzName)
			}
			return hzResources
		}).
		WithIndex(&hazelcastv1alpha1.UserCodeNamespace{}, "hazelcastResourceName", func(o client.Object) []string {
			ucn := o.(*hazelcastv1alpha1.UserCodeNamespace)
			return []string{ucn.Spec.HazelcastResourceName}
		}).
		WithIndex(&hazelcastv1alpha1.VectorCollection{}, "hazelcastResourceName", func(o client.Object) []string {
			cache := o.(*hazelcastv1alpha1.VectorCollection)
			return []string{cache.Spec.HazelcastResourceName}
		}).
		Build()
}

func FakeMtlsHttpServer(url string, tlsCfg *tls.Config, handler http.HandlerFunc) (*httptest.Server, error) {
	l, err := net.Listen("tcp", url)
	if err != nil {
		return nil, err
	}
	ts := httptest.NewUnstartedServer(handler)
	_ = ts.Listener.Close()
	ts.Listener = l
	ts.TLS = tlsCfg
	ts.StartTLS()
	return ts, nil
}

func SetupTlsConfig(k8sClient client.Client, namespace string) *tls.Config {
	certNn := types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: namespace}
	_, err := mtls.NewClient(context.Background(), k8sClient, certNn)
	Expect(err).To(BeNil())
	certSecret := &corev1.Secret{}
	Expect(k8sClient.Get(context.TODO(), certNn, certSecret)).Should(Succeed())

	ca, cert, key := certSecret.Data[mtls.TLSCAKey], certSecret.Data[corev1.TLSCertKey], certSecret.Data[corev1.TLSPrivateKeyKey]
	pool := x509.NewCertPool()
	Expect(pool.AppendCertsFromPEM(ca)).To(BeTrue())

	pair, err := tls.X509KeyPair(cert, key)
	Expect(err).Should(BeNil())

	tlsConf := &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		Certificates: []tls.Certificate{pair},
	}

	return tlsConf
}

type FakeHttpClientRegistry struct {
	clients sync.Map
}

func (hr *FakeHttpClientRegistry) Create(_ context.Context, _ client.Client, ns string) (*http.Client, error) {
	if v, ok := hr.clients.Load(types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: ns}); ok {
		return v.(*http.Client), nil
	}
	return nil, errors.New("no client found")
}

func (hr *FakeHttpClientRegistry) GetOrCreate(ctx context.Context, kubeClient client.Client, ns string) (*http.Client, error) {
	return hr.Create(ctx, kubeClient, ns)
}

func (hr *FakeHttpClientRegistry) Delete(ns string) {
	hr.clients.Delete(types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: ns})
}

func (hr *FakeHttpClientRegistry) Set(ns string, cl *http.Client) {
	hr.clients.Store(types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: ns}, cl)
}

type FakeHzClient struct {
	TOrderedMembers          []cluster.MemberInfo
	TIsClientConnected       bool
	TAreAllMembersAccessible bool
	TRunning                 bool
	TInvokeOnMember          func(ctx context.Context, req *proto.ClientMessage, uuid hztypes.UUID, opts *proto.InvokeOptions) (*proto.ClientMessage, error)
	TInvokeOnRandomTarget    func(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error)
	TShutDown                error
	TUUID                    hztypes.UUID
}

func (cl *FakeHzClient) OrderedMembers() []cluster.MemberInfo {
	return cl.TOrderedMembers
}

func (cl *FakeHzClient) IsClientConnected() bool {
	return cl.TIsClientConnected
}

func (cl *FakeHzClient) AreAllMembersAccessible() bool {
	return cl.TAreAllMembersAccessible
}

func (cl *FakeHzClient) InvokeOnMember(ctx context.Context, req *proto.ClientMessage, uuid hztypes.UUID, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
	if cl.TInvokeOnMember != nil {
		return cl.TInvokeOnMember(ctx, req, uuid, opts)
	}
	return nil, nil
}

func (cl *FakeHzClient) InvokeOnRandomTarget(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
	if cl.TInvokeOnRandomTarget != nil {
		return cl.TInvokeOnRandomTarget(ctx, req, opts)
	}
	return nil, nil
}

func (cl *FakeHzClient) Running() bool {
	return cl.TRunning
}

func (cl *FakeHzClient) ClusterId() hztypes.UUID {
	return cl.TUUID
}

func (cl *FakeHzClient) Shutdown(_ context.Context) error {
	return cl.TShutDown
}

func splitWanMapKey(key string) (hzName string, mapName string) {
	list := strings.Split(key, "__")
	return list[0], list[1]
}

func Fail(t *testing.T) func(message string, callerSkip ...int) {
	return func(message string, callerSkip ...int) {
		if len(callerSkip) > 0 {
			_, file, line, _ := runtime.Caller(callerSkip[0])
			lineInfo := fmt.Sprintf("%s:%d", file, line)
			t.Errorf("%s\n%s", lineInfo, message)
		} else {
			t.Error(message)
		}
	}
}
