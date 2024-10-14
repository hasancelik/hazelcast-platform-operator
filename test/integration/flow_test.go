package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("Flow CR", func() {
	const namespace = "default"
	const dbSecret = "db-secret"

	createSecret := func() *corev1.Secret {
		dbSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dbSecret,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"database": []byte("flow-db"),
				"username": []byte("my-user"),
				"password": []byte("strong-password"),
			},
		}
		Expect(k8sClient.Create(context.Background(), dbSecret)).Should(Succeed())
		return dbSecret
	}

	BeforeEach(func() {
		By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
		licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
		assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		dbSecret := createSecret()
		assertExists(lookupKey(dbSecret), dbSecret)
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Flow{}, &hazelcastv1alpha1.FlowList{}, namespace, map[string]string{})
		DeleteAllOf(&corev1.Secret{}, &corev1.SecretList{}, namespace, map[string]string{})
	})

	Context("Flow OPTIONS", func() {
		It("Should build correct OPTIONS", func() {
			f := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: n.LicenseKeySecret,
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: dbSecret,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OPTIONS",
							Value: "--flow.toggles.dashboard-enabled=true",
						},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), f)).Should(Succeed())
			Eventually(func() string {
				sts := getStatefulSet(f)
				envs := sts.Spec.Template.Spec.Containers[0].Env
				for _, env := range envs {
					if env.Name == "OPTIONS" {
						return env.Value
					}
				}
				return ""
			}).Should(And(
				ContainSubstring("--flow.db.username=my-user"),
				ContainSubstring("--flow.db.password=strong-password"),
				ContainSubstring("--flow.db.host=host"),
				ContainSubstring("--flow.db.database=flow-db"),
				ContainSubstring("--flow.toggles.dashboard-enabled=true"),
				ContainSubstring("--flow.hazelcast.configYamlPath=/data/hazelcast/hazelcast.yaml"),
				ContainSubstring("--flow.schema.server.clustered=true"),
			))
		})
		It("Should build OPTIONS when no Env Var is provided", func() {
			f := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: n.LicenseKeySecret,
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: dbSecret,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), f)).Should(Succeed())
			Eventually(func() string {
				sts := getStatefulSet(f)
				envs := sts.Spec.Template.Spec.Containers[0].Env
				for _, env := range envs {
					if env.Name == "OPTIONS" {
						return env.Value
					}
				}
				return ""
			}).Should(And(
				ContainSubstring("--flow.hazelcast.configYamlPath=/data/hazelcast/hazelcast.yaml"),
				ContainSubstring("--flow.schema.server.clustered=true"),
			))
		})
	})

	Context("Flow validation", func() {
		It("Should not allow hazelcast config path in the OPTIONS", func() {
			f := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: n.LicenseKeySecret,
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: dbSecret,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OPTIONS",
							Value: "--flow.hazelcast.configYamlPath=/data/hazelcast/hazelcast.yaml --flow.schema.server.clustered=true",
						},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), f)).Should(
				And(
					MatchError(ContainSubstring("Forbidden: Environment variable OPTIONS must not contain '--flow.hazelcast.configYamlPath")),
					MatchError(ContainSubstring("Forbidden: Environment variable OPTIONS must not contain '--flow.schema.server.clustered")),
				))
		})
		It("Should not allow DB configs in the OPTIONS", func() {
			f := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: n.LicenseKeySecret,
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: dbSecret,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OPTIONS",
							Value: "--flow.db.host=database",
						},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), f)).Should(
				MatchError(ContainSubstring(
					"Forbidden: Environment variable OPTIONS must not contain '--flow.db.' configs. Use 'spec.database' instead.")))
		})
		It("Should not allow DB configs with non existent secret", func() {
			f := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: n.LicenseKeySecret,
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: "do-not-exist",
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), f)).Should(MatchError(ContainSubstring("Database Secret not found")))
		})
		It("Should non existent License ley Secret", func() {
			f := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: "does-not-exist",
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: dbSecret,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), f)).Should(MatchError(ContainSubstring("Hazelcast Enterprise licenseKeySecret is not found")))
		})
	})
})
