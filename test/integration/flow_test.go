package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("Flow CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
		licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
		assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Flow{}, &hazelcastv1alpha1.FlowList{}, namespace, map[string]string{})
	})

	Context("Flow validation", func() {
		It("Should not allow hazelcast config path in the OPTIONS", func() {
			m := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: "secret",
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: "secret",
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OPTIONS",
							Value: "--flow.hazelcast.configYamlPath=/data/hazelcast/hazelcast.yaml --flow.schema.server.clustered=true",
						},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(
				And(
					MatchError(ContainSubstring("Forbidden: Environment variable OPTIONS must not contain '--flow.hazelcast.configYamlPath")),
					MatchError(ContainSubstring("Forbidden: Environment variable OPTIONS must not contain '--flow.schema.server.clustered")),
				))
		})
		It("Should not allow DB configs in the OPTIONS", func() {
			m := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: "secret",
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: "secret",
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OPTIONS",
							Value: "--flow.db.host=database",
						},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(
				MatchError(ContainSubstring(
					"Forbidden: Environment variable OPTIONS must not contain '--flow.db.' configs. Use 'spec.database' instead.")))
		})
		It("Should not allow DB configs with non existent secret", func() {
			m := &hazelcastv1alpha1.Flow{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.FlowSpec{
					LicenseKeySecretName: "secret",
					Database: hazelcastv1alpha1.DatabaseConfig{
						Host:       "host",
						SecretName: "do-not-exist",
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(MatchError(ContainSubstring("Database Secret not found")))
		})
	})
})
