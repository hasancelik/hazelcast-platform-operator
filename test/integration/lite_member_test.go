package integration

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller/hazelcast"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("Hazelcast CR with lite members", func() {
	const namespace = "default"

	BeforeEach(func() {
		By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
		licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
		assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with JVM configuration", func() {
		When("Memory is configured", func() {
			It("should set memory with percentages", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				p := ptr.To("10")
				jvmConf := &hazelcastv1alpha1.JVMConfiguration{
					Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
						InitialRAMPercentage: p,
						MinRAMPercentage:     p,
						MaxRAMPercentage:     p,
					},
				}

				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					Count: 1,
					JVM:   jvmConf,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)

				Expect(*fetchedCR.Spec.LiteMember.JVM.Memory).Should(Equal(*jvmConf.Memory))
			})

			It("should set GC params", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				s := hazelcastv1alpha1.GCTypeSerial

				gcConf := &hazelcastv1alpha1.JVMGCConfiguration{
					Logging:   ptr.To(true),
					Collector: &s,
				}

				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					Count: 1,
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						GC: gcConf,
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)

				Expect(*fetchedCR.Spec.LiteMember.JVM.GC).Should(Equal(*gcConf))
			})
		})

		When("incorrect configuration", func() {
			expectedErrStr := `%s is already set up in JVM config"`

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.InitialRamPerArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					Count: 1,
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
							InitialRAMPercentage: ptr.To("10"),
						},
						Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.InitialRamPerArg)},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.InitialRamPerArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.MinRamPerArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
							MinRAMPercentage: ptr.To("10"),
						},
						Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.MinRamPerArg)},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.MinRamPerArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.MaxRamPerArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
							MaxRAMPercentage: ptr.To("10"),
						},
						Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.MaxRamPerArg)},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.MaxRamPerArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.GCLoggingArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						GC: &hazelcastv1alpha1.JVMGCConfiguration{
							Logging: ptr.To(true),
						},
						Args: []string{fmt.Sprintf(hazelcastv1alpha1.GCLoggingArg)},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.GCLoggingArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.SerialGCArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				c := hazelcastv1alpha1.GCTypeSerial
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						Memory: nil,
						GC: &hazelcastv1alpha1.JVMGCConfiguration{
							Collector: &c,
						},
						Args: []string{fmt.Sprintf(hazelcastv1alpha1.SerialGCArg)},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.SerialGCArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.ParallelGCArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				c := hazelcastv1alpha1.GCTypeParallel
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{JVM: &hazelcastv1alpha1.JVMConfiguration{
					GC:   &hazelcastv1alpha1.JVMGCConfiguration{},
					Args: []string{fmt.Sprintf(hazelcastv1alpha1.ParallelGCArg)},
				}}
				spec.LiteMember.JVM.GC.Collector = &c

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.ParallelGCArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.G1GCArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				c := hazelcastv1alpha1.GCTypeG1
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						GC:   &hazelcastv1alpha1.JVMGCConfiguration{},
						Args: []string{fmt.Sprintf(hazelcastv1alpha1.G1GCArg)},
					},
				}
				spec.LiteMember.JVM.GC.Collector = &c

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.G1GCArg))))
			})
		})

		When("JVM arg is not configured", func() {
			It("should set the default values for JAVA_OPTS", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{Count: 1}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				assertHzStatusIsPending(hz)

				By("Checking if Hazelcast Container has the correct JAVA_OPTS")
				b := strings.Builder{}
				for k, v := range hazelcast.DefaultJavaOptions {
					b.WriteString(fmt.Sprintf(" %s=%s", k, v))
				}
				expectedJavaOpts := b.String()
				javaOpts := ""
				liteJavaOpts := ""

				ss := getStatefulSet(hz)
				for _, env := range ss.Spec.Template.Spec.Containers[0].Env {
					if env.Name == hazelcast.JavaOpts {
						javaOpts = env.Value
					}
				}

				liteSS := getStatefulSetByName(types.NamespacedName{
					Namespace: hz.Namespace,
					Name:      hz.Name + naming.LiteSuffix,
				})
				for _, env := range liteSS.Spec.Template.Spec.Containers[0].Env {
					if env.Name == hazelcast.JavaOpts {
						liteJavaOpts = env.Value
					}
				}

				Expect(javaOpts).To(ContainSubstring(expectedJavaOpts))
				Expect(liteJavaOpts).To(ContainSubstring(expectedJavaOpts))
			})
		})

		When("JVM args is configured", func() {
			It("should override the default values for JAVA_OPTS", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())

				configuredDefaults := map[string]struct{}{"-Dhazelcast.stale.join.prevention.duration.seconds": {}}
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					Count: 1,
					JVM: &hazelcastv1alpha1.JVMConfiguration{
						Args: []string{"-XX:MaxGCPauseMillis=200", "-Dhazelcast.stale.join.prevention.duration.seconds=40"},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				assertHzStatusIsPending(hz)
				ss := getStatefulSetByName(types.NamespacedName{
					Namespace: hz.Namespace,
					Name:      hz.Name + naming.LiteSuffix,
				})

				By("Checking if Hazelcast Container has the correct JAVA_OPTS")
				b := strings.Builder{}
				for _, arg := range spec.LiteMember.JVM.Args {
					b.WriteString(fmt.Sprintf(" %s", arg))
				}
				for k, v := range hazelcast.DefaultJavaOptions {
					_, ok := configuredDefaults[k]
					if !ok {
						b.WriteString(fmt.Sprintf(" %s=%s", k, v))
					}
				}
				expectedJavaOpts := b.String()
				javaOpts := ""
				for _, env := range ss.Spec.Template.Spec.Containers[0].Env {
					if env.Name == hazelcast.JavaOpts {
						javaOpts = env.Value
					}
				}

				Expect(javaOpts).To(ContainSubstring(expectedJavaOpts))
			})
		})
	})

	Context("with Resources parameters", func() {
		When("resources are given", func() {
			It("should be set to Containers' spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{Count: 1}
				spec.LiteMember.Resources = &corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("5Gi"),
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				Eventually(func() map[corev1.ResourceName]resource.Quantity {
					ss := getStatefulSetByName(types.NamespacedName{
						Namespace: hz.Namespace,
						Name:      hz.Name + naming.LiteSuffix,
					})
					return ss.Spec.Template.Spec.Containers[0].Resources.Limits
				}, timeout, interval).Should(And(
					HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")),
					HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("10Gi"))),
				)

				Eventually(func() map[corev1.ResourceName]resource.Quantity {
					ss := getStatefulSetByName(types.NamespacedName{
						Namespace: hz.Namespace,
						Name:      hz.Name + naming.LiteSuffix,
					})
					return ss.Spec.Template.Spec.Containers[0].Resources.Requests
				}, timeout, interval).Should(And(
					HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")),
					HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("5Gi"))),
				)
			})
		})
	})

	Context("with env variables", func() {
		When("configured", func() {
			It("should set them correctly", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					Count: 1,
					Env: []corev1.EnvVar{
						{
							Name:  "ENV",
							Value: "VAL",
						},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)

				Expect(fetchedCR.Spec.LiteMember.Env[0].Name).Should(Equal("ENV"))
				Expect(fetchedCR.Spec.LiteMember.Env[0].Value).Should(Equal("VAL"))

				ss := getStatefulSetByName(types.NamespacedName{
					Namespace: hz.Namespace,
					Name:      hz.Name + n.LiteSuffix,
				})

				var envs []string
				for _, e := range ss.Spec.Template.Spec.Containers[0].Env {
					envs = append(envs, e.Name)
				}
				Expect(envs).Should(ContainElement("ENV"))
			})
		})
		When("it is configured with env vars starting with HZ_", func() {
			It("should not set them", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					Count: 1,
					Env: []corev1.EnvVar{
						{
							Name:  "HZ_ENV",
							Value: "VAL",
						},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				err := k8sClient.Create(context.Background(), hz)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("Environment variables cannot start with 'HZ_'. Use customConfigCmName to configure Hazelcast."))
			})
		})
		When("it is configured with empty env var name", func() {
			It("should give an error", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{
					Count: 1,
					Env: []corev1.EnvVar{
						{
							Name:  "",
							Value: "VAL",
						},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				err := k8sClient.Create(context.Background(), hz)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("Environment variable name cannot be empty"))
			})
		})
	})

	Context("with Scheduling configuration", func() {
		When("NodeSelector is given", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{Count: 1}
				spec.LiteMember.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					NodeSelector: map[string]string{
						"node.selector": "1",
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				Eventually(func() map[string]string {
					ss := getStatefulSetByName(types.NamespacedName{
						Namespace: hz.Namespace,
						Name:      hz.Name + n.LiteSuffix,
					})
					return ss.Spec.Template.Spec.NodeSelector
				}, timeout, interval).Should(HaveKeyWithValue("node.selector", "1"))
			})
		})

		When("Affinity is given", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{Count: 1}
				spec.LiteMember.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{Key: "node.gpu", Operator: corev1.NodeSelectorOpExists},
										},
									},
								},
							},
						},
						PodAffinity: &corev1.PodAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 10,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "node.zone",
									},
								},
							},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 10,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "node.zone",
									},
								},
							},
						},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				Eventually(func() *corev1.Affinity {
					ss := getStatefulSetByName(types.NamespacedName{
						Namespace: hz.Namespace,
						Name:      hz.Name + n.LiteSuffix,
					})
					return ss.Spec.Template.Spec.Affinity
				}, timeout, interval).Should(Equal(spec.LiteMember.Scheduling.Affinity))
			})
		})

		When("Toleration is given", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LiteMember = &hazelcastv1alpha1.LiteMember{Count: 1}
				spec.LiteMember.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					Tolerations: []corev1.Toleration{
						{
							Key:      "node.zone",
							Operator: corev1.TolerationOpExists,
						},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				Eventually(func() []corev1.Toleration {
					ss := getStatefulSetByName(types.NamespacedName{
						Namespace: hz.Namespace,
						Name:      hz.Name + n.LiteSuffix,
					})
					return ss.Spec.Template.Spec.Tolerations
				}, timeout, interval).Should(Equal(spec.LiteMember.Scheduling.Tolerations))
			})
		})
	})
})
