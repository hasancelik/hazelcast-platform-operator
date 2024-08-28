package config

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/faketest"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		src  map[string]any
		dst  map[string]any
		want map[string]any
	}{
		// empty
		{
			src:  make(map[string]any),
			dst:  make(map[string]any),
			want: make(map[string]any),
		},
		// union
		{
			src: map[string]any{
				"foo": "foo",
			},
			dst: map[string]any{
				"bar": "bar",
			},
			want: map[string]any{
				"foo": "foo",
				"bar": "bar",
			},
		},
		// map merge
		{
			src: map[string]any{
				"foo": map[string]any{
					"bar": "bar",
				},
			},
			dst: map[string]any{
				"foo": map[string]any{
					"baz": "baz",
				},
			},
			want: map[string]any{
				"foo": map[string]any{
					"bar": "bar",
					"baz": "baz",
				},
			},
		},
		// value overwrite
		{
			src: map[string]any{
				"foo": "bar",
			},
			dst: map[string]any{
				"foo": "baz",
			},
			want: map[string]any{
				"foo": "bar",
			},
		},
		// example
		{
			src: map[string]any{
				"hazelcast": map[string]any{
					"persistence": map[string]any{
						"enabled": true,
					},
					"map": map[string]any{
						"test-map": map[string]any{
							"data-persistence": map[string]any{
								"enabled": true,
								"fsync":   true,
							},
						},
					},
				},
			},
			dst: map[string]any{
				"hazelcast": map[string]any{
					"persistence": map[string]any{
						"enabled":    true,
						"base-dir":   "/mnt/persistence",
						"backup-dir": "/mnt/hot-backup",
					},
					"map": map[string]any{
						"backup-count": 1,
					},
				},
			},
			want: map[string]any{
				"hazelcast": map[string]any{
					"persistence": map[string]any{
						"enabled":    true,
						"base-dir":   "/mnt/persistence",
						"backup-dir": "/mnt/hot-backup",
					},
					"map": map[string]any{
						"backup-count": 1,
						"test-map": map[string]any{
							"data-persistence": map[string]any{
								"enabled": true,
								"fsync":   true,
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		deepMerge(tc.dst, tc.src)
		if diff := cmp.Diff(tc.want, tc.dst); diff != "" {
			t.Errorf("deepMerge(%v, %v) mismatch (-want, +got):\n%s", tc.dst, tc.src, diff)
		}
	}
}

func Test_hazelcastConfig(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		hzSpec         hazelcastv1alpha1.HazelcastSpec
		expectedResult interface{}
		actualResult   func(h Hazelcast) interface{}
	}{
		{
			name:           "Empty Compact Serialization",
			hzSpec:         hazelcastv1alpha1.HazelcastSpec{},
			expectedResult: (*CompactSerialization)(nil),
			actualResult: func(h Hazelcast) interface{} {
				return h.Serialization.CompactSerialization
			},
		},
		{
			name: "Compact Serialization Class",
			hzSpec: hazelcastv1alpha1.HazelcastSpec{
				Serialization: &hazelcastv1alpha1.SerializationConfig{
					CompactSerialization: &hazelcastv1alpha1.CompactSerializationConfig{
						Classes: []string{"test1", "test2"},
					},
				},
			},
			expectedResult: []string{
				"class: test1",
				"class: test2",
			},
			actualResult: func(h Hazelcast) interface{} {
				return h.Serialization.CompactSerialization.Classes
			},
		},
		{
			name: "Compact Serialization Serializer",
			hzSpec: hazelcastv1alpha1.HazelcastSpec{
				Serialization: &hazelcastv1alpha1.SerializationConfig{
					CompactSerialization: &hazelcastv1alpha1.CompactSerializationConfig{
						Serializers: []string{"test1", "test2"},
					},
				},
			},
			expectedResult: []string{
				"serializer: test1",
				"serializer: test2",
			},
			actualResult: func(h Hazelcast) interface{} {
				return h.Serialization.CompactSerialization.Serializers
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RegisterFailHandler(faketest.Fail(t))
			h := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hazelcast",
					Namespace: "default",
				},
				Spec: test.hzSpec,
			}

			c := faketest.FakeK8sClient(h)
			data, err := HazelcastConfig(context.Background(), c, h, logr.Discard())
			if err != nil {
				t.Errorf("Error retrieving Secret data")
			}
			actualConfig := &HazelcastWrapper{}
			err = yaml.Unmarshal(data, actualConfig)
			if err != nil {
				t.Errorf("Error on unmarshal actual Hazelcast config YAML")
			}

			Expect(test.actualResult(actualConfig.Hazelcast)).Should(Equal(test.expectedResult))
		})
	}
}

func Test_hazelcastConfigMultipleCRs(t *testing.T) {
	meta := metav1.ObjectMeta{
		Name:      "hazelcast",
		Namespace: "default",
	}
	cm := &corev1.Secret{
		ObjectMeta: meta,
	}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: meta,
	}

	hzConfig := &HazelcastWrapper{}
	err := yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
	if err != nil {
		t.Errorf("Error on unmarshal Hazelcast config")
	}
	structureSpec := hazelcastv1alpha1.DataStructureSpec{
		HazelcastResourceName: meta.Name,
		BackupCount:           ptr.To(int32(1)),
		AsyncBackupCount:      0,
	}
	structureStatus := hazelcastv1alpha1.DataStructureStatus{State: hazelcastv1alpha1.DataStructureSuccess}

	tests := []struct {
		name     string
		listKeys listKeys
		c        client.Object
	}{
		{
			name: "Cache CRs",
			listKeys: func(h Hazelcast) []string {
				return getKeys(h.Cache)
			},
			c: &hazelcastv1alpha1.Cache{
				TypeMeta: metav1.TypeMeta{
					Kind: "Cache",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.CacheSpec{DataStructureSpec: structureSpec},
				Status: hazelcastv1alpha1.CacheStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "Topic CRs",
			listKeys: func(h Hazelcast) []string {
				return getKeys(h.Topic)
			},
			c: &hazelcastv1alpha1.Topic{
				TypeMeta: metav1.TypeMeta{
					Kind: "Topic",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.TopicSpec{HazelcastResourceName: meta.Name},
				Status: hazelcastv1alpha1.TopicStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "MultiMap CRs",
			listKeys: func(h Hazelcast) []string {
				return getKeys(h.MultiMap)
			},
			c: &hazelcastv1alpha1.MultiMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "MultiMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.MultiMapSpec{DataStructureSpec: structureSpec},
				Status: hazelcastv1alpha1.MultiMapStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "ReplicatedMap CRs",
			listKeys: func(h Hazelcast) []string {
				return getKeys(h.ReplicatedMap)
			},
			c: &hazelcastv1alpha1.ReplicatedMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ReplicatedMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.ReplicatedMapSpec{HazelcastResourceName: meta.Name, AsyncFillup: ptr.To(true)},
				Status: hazelcastv1alpha1.ReplicatedMapStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "Queue CRs",
			listKeys: func(h Hazelcast) []string {
				return getKeys(h.Queue)
			},
			c: &hazelcastv1alpha1.Queue{
				TypeMeta: metav1.TypeMeta{
					Kind: "Queue",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.QueueSpec{
					EmptyQueueTtlSeconds: ptr.To(int32(10)),
					MaxSize:              0,
					DataStructureSpec:    structureSpec,
				},
				Status: hazelcastv1alpha1.QueueStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "Map CRs",
			listKeys: func(h Hazelcast) []string {
				return getKeys(h.Map)
			},
			c: &hazelcastv1alpha1.Map{
				TypeMeta: metav1.TypeMeta{
					Kind: "Map",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: structureSpec,
					TimeToLiveSeconds: 10,
					Eviction: hazelcastv1alpha1.EvictionConfig{
						MaxSize: 0,
					},
				},
				Status: hazelcastv1alpha1.MapStatus{State: hazelcastv1alpha1.MapSuccess},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RegisterFailHandler(faketest.Fail(t))
			crNames := []string{"cr-name-1", "another-created-cr", "custom-resource"}
			objects := make([]client.Object, len(crNames))
			for i := range crNames {
				test.c.SetName(crNames[i])
				objects[i] = test.c.DeepCopyObject().(client.Object)
			}
			objects = append(objects, cm, h)
			c := faketest.FakeK8sClient(objects...)
			data, err := HazelcastConfig(context.Background(), c, h, logr.Discard())
			if err != nil {
				t.Errorf("Error retrieving Secret data")
			}
			actualConfig := &HazelcastWrapper{}
			err = yaml.Unmarshal(data, actualConfig)
			if err != nil {
				t.Errorf("Error on unmarshal actual Hazelcast config YAML")
			}
			listKeys := test.listKeys(actualConfig.Hazelcast)
			Expect(listKeys).Should(HaveLen(len(crNames)))
			for _, name := range crNames {
				Expect(listKeys).Should(ContainElement(name))
			}
		})
	}
}

type listKeys func(h Hazelcast) []string

func getKeys[C Cache | ReplicatedMap | MultiMap | Topic | Queue | Map](m map[string]C) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func Test_hazelcastConfigMultipleWanCRs(t *testing.T) {
	hz := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hazelcast",
			Namespace: "default",
		},
	}
	objects := []client.Object{
		hz,
		&hazelcastv1alpha1.Map{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "map",
				Namespace: "default",
			},
			Spec: hazelcastv1alpha1.MapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hz.Name,
				},
			},
		},
		&hazelcastv1alpha1.WanReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wan-1",
				Namespace: "default",
			},
			Spec: hazelcastv1alpha1.WanReplicationSpec{
				Resources: []hazelcastv1alpha1.ResourceSpec{
					{
						Name: hz.Name,
						Kind: hazelcastv1alpha1.ResourceKindHZ,
					},
				},
				TargetClusterName: "dev",
				Endpoints:         "10.0.0.1:5701",
			},
			Status: hazelcastv1alpha1.WanReplicationStatus{
				WanReplicationMapsStatus: map[string]hazelcastv1alpha1.WanReplicationMapStatus{
					hz.Name + "__map": {
						PublisherId: "map-wan-1",
						Status:      hazelcastv1alpha1.WanStatusSuccess,
					},
				},
			},
		},
		&hazelcastv1alpha1.WanReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wan-2",
				Namespace: "default",
			},
			Spec: hazelcastv1alpha1.WanReplicationSpec{
				Resources: []hazelcastv1alpha1.ResourceSpec{
					{
						Name: hz.Name,
						Kind: hazelcastv1alpha1.ResourceKindHZ,
					},
				},
				TargetClusterName: "dev",
				Endpoints:         "10.0.0.2:5701",
			},
			Status: hazelcastv1alpha1.WanReplicationStatus{
				WanReplicationMapsStatus: map[string]hazelcastv1alpha1.WanReplicationMapStatus{
					hz.Name + "__map": {
						PublisherId: "map-wan-2",
						Status:      hazelcastv1alpha1.WanStatusSuccess,
					},
				},
			},
		},
	}
	c := faketest.FakeK8sClient(objects...)

	bytes, err := HazelcastConfig(context.TODO(), c, hz, logr.Discard())
	if err != nil {
		t.Errorf("unable to build Config, %e", err)
	}
	hzConfig := &HazelcastWrapper{}
	err = yaml.Unmarshal(bytes, hzConfig)
	if err != nil {
		t.Error(err)
	}
	wrConf, ok := hzConfig.Hazelcast.WanReplication["map-default"]
	if !ok {
		t.Errorf("wan config for map-default not found")
	}
	if _, ok := wrConf.BatchPublisher["map-wan-1"]; !ok {
		t.Errorf("butch publisher map-wan-1 not found")
	}
	if _, ok := wrConf.BatchPublisher["map-wan-2"]; !ok {
		t.Errorf("butch publisher map-wan-2 not found")
	}
}

func Test_CustomConfig_SecurityEnabled(t *testing.T) {
	tests := []struct {
		name         string
		configMutate func(map[string]interface{}) map[string]interface{}
		valid        bool
	}{
		{
			name:         "security configured",
			configMutate: func(m map[string]interface{}) map[string]interface{} { return m },
			valid:        true,
		},
		{
			name: "missing client-permissions",
			configMutate: func(m map[string]interface{}) map[string]interface{} {
				m["client-permissions"] = nil
				return m
			},
			valid: false,
		},
		{
			name: "missing all client-permissions",
			configMutate: func(m map[string]interface{}) map[string]interface{} {
				cp := make(map[string]interface{})
				cp["map"] = make(map[string]interface{})
				cp["something"] = make(map[string]interface{})
				m["client-permissions"] = cp
				return m
			},
			valid: false,
		},
		{
			name: "missing realms",
			configMutate: func(m map[string]interface{}) map[string]interface{} {
				m["realms"] = nil
				return m
			},
			valid: false,
		},
		{
			name: "wrong client realm",
			configMutate: func(m map[string]interface{}) map[string]interface{} {
				ca := make(map[string]interface{})
				ca["realm"] = "differentOne"
				m["client-authentication"] = ca
				return m
			},
			valid: false,
		},
		{
			name: "client realm is not simple",
			configMutate: func(m map[string]interface{}) map[string]interface{} {
				realm := make(map[string]interface{})
				realm["name"] = "simpleRealm"
				auth := make(map[string]interface{})
				jaas := make(map[string]interface{})
				users := make([]map[string]interface{}, 0)
				usr := make(map[string]interface{})
				usr["username"] = "password"
				usr["password"] = "password"
				usr["roles"] = []string{"root"}
				users = append(users, usr)
				jaas["users"] = users
				auth["jaas"] = jaas
				realm["authentication"] = auth
				realms := make([]map[string]interface{}, 0)
				realms = append(realms, realm)
				m["realms"] = realms
				return m
			},
			valid: false,
		},
		{
			name: "user do not have role",
			configMutate: func(m map[string]interface{}) map[string]interface{} {
				realm := make(map[string]interface{})
				realm["name"] = "simpleRealm"
				auth := make(map[string]interface{})
				simp := make(map[string]interface{})
				users := make([]map[string]interface{}, 0)
				usr := make(map[string]interface{})
				usr["username"] = "password"
				usr["password"] = "password"
				usr["roles"] = []string{"role1"}
				users = append(users, usr)
				simp["users"] = users
				auth["simple"] = simp
				realm["authentication"] = auth
				realms := make([]map[string]interface{}, 0)
				realms = append(realms, realm)
				m["realms"] = realms
				return m
			},
			valid: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nn := types.NamespacedName{Name: "hazelcast", Namespace: "default"}
			ccSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-config-secret",
					Namespace: nn.Namespace,
				},
			}
			h := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nn.Name,
					Namespace: nn.Namespace,
				},
				Spec: hazelcastv1alpha1.HazelcastSpec{
					CustomConfig: hazelcastv1alpha1.CustomConfig{
						SecretName: ccSecret.Name,
					},
				},
			}
			customConfig := make(map[string]interface{})
			sc := make(map[string]interface{})
			sc["enabled"] = true

			ca := make(map[string]string)
			ca["realm"] = "simpleRealm"
			sc["client-authentication"] = ca

			clp := make(map[string]interface{})
			clpall := make(map[string]interface{})
			clpall["principal"] = "root"
			clp["all"] = clpall
			sc["client-permissions"] = clp

			realms := make([]map[string]interface{}, 0)
			simpleRealm := make(map[string]interface{})
			simpleRealm["name"] = "simpleRealm"
			authentication := make(map[string]interface{})
			simple := make(map[string]interface{})
			users := make([]map[string]interface{}, 0)
			user := make(map[string]interface{})
			user["roles"] = []string{"role1", "role2", "root"}
			users = append(users, user)
			simple["users"] = users
			authentication["simple"] = simple
			simpleRealm["authentication"] = authentication
			realms = append(realms, simpleRealm)
			sc["realms"] = realms

			sc = test.configMutate(sc)
			customConfig["security"] = sc

			out, err := yaml.Marshal(customConfig)
			if err != nil {
				t.Errorf("unable to marshal Custom Config")
			}
			ccSecret.Data = make(map[string][]byte)
			ccSecret.Data[n.HazelcastCustomConfigKey] = out

			c := faketest.FakeK8sClient(ccSecret, h)

			_, err = HazelcastConfig(context.TODO(), c, h, logr.Discard())
			if test.valid && err != nil {
				t.Errorf("expect to configuration to be valid")
			}
			if !test.valid && err == nil {
				t.Errorf("expect to validate missing user permissions")
			}
		})
	}
}
