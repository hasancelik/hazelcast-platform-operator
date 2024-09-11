package e2e

import (
	"context"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

const (
	logInterval = 10 * Millisecond
)

var _ = Describe("Hazelcast", Group("hz"), func() {
	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Cluster creation", func() {
		It("should create a Hazelcast cluster with a custom name", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("h-1")
			hazelcast := hazelcastconfig.ClusterName(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)
			assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
			evaluateReadyMembers(hzLookupKey)
			assertMemberLogs(hazelcast, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				deletePods(hzLookupKey)
				evaluateReadyMembers(hzLookupKey)
			})
		})
		It("should update ready members status in Hazelcast cluster", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("h-2")
			hazelcast := hazelcastconfig.Default(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)
			assertMembersNotRestarted(hzLookupKey)
			assertMemberLogs(hazelcast, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				deletePods(hzLookupKey)
				evaluateReadyMembers(hzLookupKey)
			})
		})

		It("should update detailed members status in Hazelcast cluster", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("h-3")
			hazelcast := hazelcastconfig.Default(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			hz := &hazelcastcomv1alpha1.Hazelcast{}
			memberStateT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) string {
				return string(status.State)
			}
			masterT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) bool {
				return status.Master
			}
			Eventually(func() []hazelcastcomv1alpha1.HazelcastMemberStatus {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Members
			}, 30*Second, interval).Should(And(HaveLen(3),
				ContainElement(WithTransform(memberStateT, Equal("ACTIVE"))),
				ContainElement(WithTransform(masterT, Equal(true))),
			))
		})

		It("should validate correct pod names and IPs for Hazelcast members", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("h-4")
			hazelcast := hazelcastconfig.Default(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			hz := &hazelcastcomv1alpha1.Hazelcast{}
			err := k8sClient.Get(context.Background(), hzLookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			By("checking hazelcast members pod name")
			for _, member := range hz.Status.Members {
				Expect(member.PodName).Should(ContainSubstring(hz.Name))
				pod := &corev1.Pod{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: hz.Namespace, Name: member.PodName}, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(member.Ip).Should(Equal(pod.Status.PodIP))
			}
		})
	})

	Context("Handling errors", func() {
		assertStatusAndMessageEventually := func(phase hazelcastcomv1alpha1.Phase) {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Phase
			}, 3*Minute, interval).Should(Equal(phase))
			Expect(hz.Status.Message).Should(Not(BeEmpty()))
		}

		It("should reflect external API errors in Hazelcast CR status", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("h-5")
			CreateHazelcastCRWithoutCheck(hazelcastconfig.Faulty(hzLookupKey, labels))
			assertStatusAndMessageEventually(hazelcastcomv1alpha1.Failed)
		})
	})

	Context("Cluster deletion", func() {
		It("should delete dependent data structures and backups on Hazelcast CR deletion", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("h-6")
			clusterSize := int32(3)

			hz := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			CreateHazelcastCR(hz)
			evaluateReadyMembers(hzLookupKey)

			m := hazelcastconfig.DefaultMap(mapLookupKey, hz.Name, labels)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			mm := hazelcastconfig.DefaultMultiMap(mmLookupKey, hz.Name, labels)
			Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
			assertDataStructureStatus(mmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.MultiMap{})

			rm := hazelcastconfig.DefaultReplicatedMap(rmLookupKey, hz.Name, labels)
			Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
			assertDataStructureStatus(rmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.ReplicatedMap{})

			topic := hazelcastconfig.DefaultTopic(topicLookupKey, hz.Name, labels)
			Expect(k8sClient.Create(context.Background(), topic)).Should(Succeed())
			assertDataStructureStatus(topicLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Topic{})

			DeleteAllOf(hz, &hazelcastcomv1alpha1.HazelcastList{}, hz.Namespace, labels)

			err := k8sClient.Get(context.Background(), mapLookupKey, m)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			err = k8sClient.Get(context.Background(), topicLookupKey, topic)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("TLS Configuration", func() {
		DescribeTable("should form a cluster with TLS configuration enabled", func(useCertChain bool) {
			setLabelAndCRName("h-7")
			hz := hazelcastconfig.HazelcastTLS(hzLookupKey, labels)

			tlsSecretNn := types.NamespacedName{
				Name:      hz.Spec.TLS.SecretName,
				Namespace: hz.Namespace,
			}
			secret := hazelcastconfig.TLSSecret(tlsSecretNn, map[string]string{}, useCertChain)
			By("creating TLS secret", func() {
				Eventually(func() error {
					return k8sClient.Create(context.Background(), secret)
				}, Minute, interval).Should(Succeed())
				assertExists(tlsSecretNn, &corev1.Secret{})
			})

			CreateHazelcastCR(hz)
			evaluateReadyMembers(hzLookupKey)
		},
			Entry("using single certificate", Tag(Kind|AnyCloud), false),
			Entry("using certificate chain", Tag(Kind|AnyCloud), true))

		DescribeTable("should support mutual TLS authentication in Hazelcast cluster", func(useCertChain bool) {
			setLabelAndCRName("h-8")
			hz := hazelcastconfig.HazelcastMTLS(hzLookupKey, labels, false)

			tlsSecretNn := types.NamespacedName{
				Name:      hz.Spec.TLS.SecretName,
				Namespace: hz.Namespace,
			}
			secret := hazelcastconfig.TLSSecret(tlsSecretNn, map[string]string{}, useCertChain)
			By("creating TLS secret", func() {
				Eventually(func() error {
					return k8sClient.Create(context.Background(), secret)
				}, Minute, interval).Should(Succeed())
				assertExists(tlsSecretNn, &corev1.Secret{})
			})

			CreateHazelcastCR(hz)
			evaluateReadyMembers(hzLookupKey)
		},
			Entry("using single certificate", Tag(Kind|AnyCloud), false),
			Entry("using certificate chain", Tag(Kind|AnyCloud), true))

		DescribeTable("should form a cluster when mutual TLS authentication configured as optional", func(useCertChain bool) {
			setLabelAndCRName("h-8")
			hz := hazelcastconfig.HazelcastMTLS(hzLookupKey, labels, true)

			tlsSecretNn := types.NamespacedName{
				Name:      hz.Spec.TLS.SecretName,
				Namespace: hz.Namespace,
			}
			secret := hazelcastconfig.TLSSecret(tlsSecretNn, map[string]string{}, useCertChain)
			By("creating TLS secret", func() {
				Eventually(func() error {
					return k8sClient.Create(context.Background(), secret)
				}, Minute, interval).Should(Succeed())
				assertExists(tlsSecretNn, &corev1.Secret{})
			})

			CreateHazelcastCR(hz)
			evaluateReadyMembers(hzLookupKey)
		},
			Entry("using single certificate", Tag(Kind|AnyCloud), false),
			Entry("using certificate chain", Tag(Kind|AnyCloud), true))
	})

	Context("Custom Config", func() {
		It("should setup client authentication from Custom Config", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("cc-1")

			hz := hazelcastconfig.Default(hzLookupKey, labels)
			ccSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-config-secret",
					Namespace: hz.Namespace,
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
			user["username"] = "admin"
			user["password"] = "admin-pass"
			user["roles"] = []string{"role1", "role2", "root"}
			users = append(users, user)
			simple["users"] = users
			authentication["simple"] = simple
			simpleRealm["authentication"] = authentication
			realms = append(realms, simpleRealm)
			sc["realms"] = realms

			customConfig["security"] = sc
			out, err := yaml.Marshal(customConfig)
			Expect(err).Should(Succeed())

			ccSecret.Data = make(map[string][]byte)
			ccSecret.Data[n.HazelcastCustomConfigKey] = out

			Eventually(func() error {
				err := k8sClient.Create(context.TODO(), ccSecret)
				if errors.IsAlreadyExists(err) {
					return nil
				}
				return err
			}, Minute, interval).Should(Succeed())

			hz.Spec.CustomConfig.SecretName = ccSecret.Name
			CreateHazelcastCR(hz)
			evaluateReadyMembers(hzLookupKey)
		})
	})
})
