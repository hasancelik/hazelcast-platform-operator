package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("Hazelcast Config Secret", func() {
	const namespace = "default"

	FetchConfigSecret := func(h *hazelcastv1alpha1.Hazelcast) *v1.Secret {
		sc := &v1.Secret{}
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, sc)).Should(Succeed())
		return sc
	}

	GetHzConfig := func(h *hazelcastv1alpha1.Hazelcast) map[string]interface{} {
		s := FetchConfigSecret(h)
		expectedMap := make(map[string]interface{})
		Expect(yaml.Unmarshal(s.Data["hazelcast.yaml"], expectedMap)).Should(Succeed())
		return expectedMap[n.HazelcastCustomConfigKey].(map[string]interface{})
	}

	BeforeEach(func() {
		By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
		licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
		assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with custom configs", func() {
		createCm := func(meta metav1.ObjectMeta, data interface{}) client.Object {
			cm := &v1.ConfigMap{
				ObjectMeta: meta,
			}
			if data == nil {
				return cm
			}
			out, err := yaml.Marshal(data)
			Expect(err).To(BeNil())
			cm.Data = make(map[string]string)
			cm.Data[n.HazelcastCustomConfigKey] = string(out)
			return cm
		}

		createSecret := func(meta metav1.ObjectMeta, data interface{}) client.Object {
			sc := &v1.Secret{
				ObjectMeta: meta,
			}
			if data == nil {
				return sc
			}
			out, err := yaml.Marshal(data)
			Expect(err).To(BeNil())
			sc.StringData = make(map[string]string)
			sc.StringData[n.HazelcastCustomConfigKey] = string(out)
			return sc
		}

		DescribeTable("should add new section to config",
			func(ccObjF func(meta metav1.ObjectMeta, data interface{}) client.Object) {
				customConfig := make(map[string]interface{})
				sc := make(map[string]interface{})
				sc["portable-version"] = 0
				sc["use-native-byte-order"] = false
				sc["byte-order"] = "BIG_ENDIAN"
				sc["check-class-def-errors"] = true
				customConfig["serialization"] = sc
				ccObj := ccObjF(randomObjectMeta(namespace), customConfig)
				Expect(k8sClient.Create(context.Background(), ccObj)).Should(Succeed())
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				switch ccObj.(type) {
				case *v1.Secret:
					hz.Spec.CustomConfig.SecretName = ccObj.GetName()
				case *v1.ConfigMap:
					hz.Spec.CustomConfig.ConfigMapName = ccObj.GetName()
				}
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
				assertHzStatusIsPending(hz)

				hzConfig := GetHzConfig(hz)
				Expect(hzConfig).Should(And(
					HaveKey("advanced-network"), HaveKey("cluster-name"), HaveKey("jet"), HaveKey("serialization")))
				expectedSer := hzConfig["serialization"].(map[string]interface{})
				Expect(expectedSer).To(Equal(sc))
			},
			Entry("with ConfigMap custom config", createCm),
			Entry("with Secret custom config", createSecret),
		)

		DescribeTable("should not override CR configs",
			func(ccObjF func(meta metav1.ObjectMeta, data interface{}) client.Object) {
				customConfig := make(map[string]interface{})
				ucdConf := make(map[string]interface{})
				ucdConf["enabled"] = true
				ucdConf["class-cache-mode"] = "ETERNAL"
				ucdConf["provider-mode"] = "LOCAL_AND_CACHED_CLASSES"
				ucdConf["blacklist-prefixes"] = "com.foo,com.bar"
				ucdConf["whitelist-prefixes"] = "com.bar.MyClass"
				ucdConf["provider-filter"] = "HAS_ATTRIBUTE:lite"
				customConfig["user-code-deployment"] = ucdConf
				ccObj := ccObjF(randomObjectMeta(namespace), customConfig)
				Expect(k8sClient.Create(context.Background(), ccObj)).Should(Succeed())
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.DeprecatedUserCodeDeployment = &hazelcastv1alpha1.UserCodeDeploymentConfig{
					ClientEnabled: pointer.Bool(false),
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				switch ccObj.(type) {
				case *v1.Secret:
					hz.Spec.CustomConfig.SecretName = ccObj.GetName()
				case *v1.ConfigMap:
					hz.Spec.CustomConfig.ConfigMapName = ccObj.GetName()
				}
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
				assertHzStatusIsPending(hz)

				hzConfig := GetHzConfig(hz)
				Expect(hzConfig).Should(HaveKey("user-code-deployment"))
				Expect(hzConfig["user-code-deployment"]).Should(HaveKeyWithValue("enabled", false))
				Expect(hzConfig["user-code-deployment"]).Should(And(
					Not(HaveKey("class-cache-mode")), Not(HaveKey("provider-filter")), Not(HaveKey("provider-mode"))))
			},
			Entry("with ConfigMap custom config", createCm),
			Entry("with Secret custom config", createSecret),
		)

		DescribeTable("should not override advanced network config",
			func(ccObjF func(meta metav1.ObjectMeta, data interface{}) client.Object) {
				customConfig := make(map[string]interface{})
				anConf := make(map[string]interface{})
				anConf["enabled"] = false
				customConfig["advanced-network"] = anConf
				ccObj := ccObjF(randomObjectMeta(namespace), customConfig)
				Expect(k8sClient.Create(context.Background(), ccObj)).Should(Succeed())
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				switch ccObj.(type) {
				case *v1.Secret:
					hz.Spec.CustomConfig.SecretName = ccObj.GetName()
				case *v1.ConfigMap:
					hz.Spec.CustomConfig.ConfigMapName = ccObj.GetName()
				}
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
				assertHzStatusIsPending(hz)

				hzConfig := GetHzConfig(hz)
				Expect(hzConfig).Should(HaveKey("advanced-network"))
				Expect(hzConfig["advanced-network"]).Should(HaveKeyWithValue("enabled", true))
				Expect(hzConfig["advanced-network"]).Should(And(
					HaveKey("client-server-socket-endpoint-config"),
					HaveKey("member-server-socket-endpoint-config"),
					HaveKey("rest-server-socket-endpoint-config")))
			},
			Entry("with ConfigMap custom config", createCm),
			Entry("with Secret custom config", createSecret),
		)

		DescribeTable("should fail if the object does not exist", func(obj client.Object) {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
			}
			switch obj.(type) {
			case *v1.Secret:
				hz.Spec.CustomConfig.SecretName = "scrt"
			case *v1.ConfigMap:
				hz.Spec.CustomConfig.ConfigMapName = "cm"
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("for Hazelcast custom configs not found")))
		},
			Entry("with ConfigMap custom config", &v1.ConfigMap{}),
			Entry("with Secret custom config", &v1.Secret{}),
		)

		DescribeTable("should fail if the config map does not contain the expected key",
			func(ccObjF func(meta metav1.ObjectMeta, data interface{}) client.Object) {
				ccObj := ccObjF(randomObjectMeta(namespace), nil)
				Expect(k8sClient.Create(context.Background(), ccObj)).Should(Succeed())
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				switch ccObj.(type) {
				case *v1.Secret:
					hz.Spec.CustomConfig.SecretName = ccObj.GetName()
				case *v1.ConfigMap:
					hz.Spec.CustomConfig.ConfigMapName = ccObj.GetName()
				}
				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("for Hazelcast custom configs must contain 'hazelcast' key")))
			},
			Entry("with ConfigMap custom config", createCm),
			Entry("with Secret custom config", createSecret),
		)

		DescribeTable("should fail if the value in the config map is not a valid yaml",
			func(ccObjF func(meta metav1.ObjectMeta, data interface{}) client.Object) {
				ccObj := ccObjF(randomObjectMeta(namespace), "it: is: invalid: yaml")
				Expect(k8sClient.Create(context.Background(), ccObj)).Should(Succeed())
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				switch ccObj.(type) {
				case *v1.Secret:
					hz.Spec.CustomConfig.SecretName = ccObj.GetName()
				case *v1.ConfigMap:
					hz.Spec.CustomConfig.ConfigMapName = ccObj.GetName()
				}
				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("for Hazelcast custom configs is not a valid yaml")))
			},
			Entry("with ConfigMap custom config", createCm),
			Entry("with Secret custom config", createSecret),
		)

		It("should not allow the usage of ConfigMap and Secret together", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
			}
			hz.Spec.CustomConfig.SecretName = "scrt"
			hz.Spec.CustomConfig.ConfigMapName = "cm"
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("ConfigMap and Secret cannot be used for Custom Config at the same time.")))
		})
	})
})
