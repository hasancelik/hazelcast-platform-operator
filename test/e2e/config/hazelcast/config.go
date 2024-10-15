package hazelcast

import (
	"flag"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var (
	hazelcastVersion = flag.String("hazelcast-version", naming.HazelcastVersion, "Default Hazelcast version used in e2e tests")
	hazelcastEERepo  = flag.String("hazelcast-ee-repo", naming.HazelcastEERepo, "Enterprise Hazelcast repository used in e2e tests")
	agentRepo        = flag.String("agent-repo", naming.AgentRepo, "Agent repository used in e2e tests")
	agentVersion     = flag.String("agent-version", naming.AgentVersion, "Agent version used in e2e tests")
	StorageClass     = flag.String("storage-class", "", "Storage class name to be used with different cloud providers")
)

var (
	ClusterName = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				ClusterName:          "development",
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			},
		}
	}

	Default = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			},
		}
	}

	WithLiteMembers = func(lk types.NamespacedName, lbls map[string]string, count int32) *hazelcastcomv1alpha1.Hazelcast {
		hz := Default(lk, lbls)
		hz.Spec.LiteMember = &hazelcastcomv1alpha1.LiteMember{
			Count: count,
		}
		return hz
	}

	ExposeExternallySmartLoadBalancer = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
				},
			},
		}
	}

	ExposeExternallySmartNodePort = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastcomv1alpha1.MemberAccessNodePortExternalIP,
				},
			},
		}
	}

	ExposeExternallySmartNodePortNodeName = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeNodePort,
					MemberAccess:         hazelcastcomv1alpha1.MemberAccessNodePortNodeName,
				},
			},
		}
	}

	ExposeExternallyUnisocket = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	HazelcastPersistencePVC = func(lk types.NamespacedName, clusterSize int32, labels map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(clusterSize),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
						StorageClassName: StorageClass,
					},
				},
			},
		}
	}

	CPSubsystem = func(clusterSize int32) hazelcastcomv1alpha1.HazelcastSpec {
		return hazelcastcomv1alpha1.HazelcastSpec{
			Agent: hazelcastcomv1alpha1.AgentConfiguration{
				Repository: *agentRepo,
				Version:    *agentVersion,
			},
			ClusterSize:          ptr.To(clusterSize),
			Repository:           *hazelcastEERepo,
			LicenseKeySecretName: naming.LicenseKeySecret,
			Version:              naming.HazelcastVersion,
			LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			CPSubsystem: &hazelcastcomv1alpha1.CPSubsystem{
				PVC: &hazelcastcomv1alpha1.PvcConfiguration{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					StorageClassName: StorageClass,
				},
			},
		}
	}

	CPSubsystemPersistence = func(clusterSize int32) hazelcastcomv1alpha1.HazelcastSpec {
		return hazelcastcomv1alpha1.HazelcastSpec{
			Agent: hazelcastcomv1alpha1.AgentConfiguration{
				Repository: *agentRepo,
				Version:    *agentVersion,
			},
			ClusterSize:          ptr.To(clusterSize),
			Version:              naming.HazelcastVersion,
			Repository:           *hazelcastEERepo,
			LicenseKeySecretName: naming.LicenseKeySecret,
			LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
				PVC: &hazelcastcomv1alpha1.PvcConfiguration{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					StorageClassName: StorageClass,
				},
			},
			CPSubsystem: &hazelcastcomv1alpha1.CPSubsystem{},
		}
	}

	HazelcastRestore = func(hz *hazelcastcomv1alpha1.Hazelcast, restoreConfig hazelcastcomv1alpha1.RestoreConfiguration) *hazelcastcomv1alpha1.Hazelcast {
		hzRestore := &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      hz.Name,
				Namespace: hz.Namespace,
				Labels:    hz.Labels,
			},
			Spec: hz.Spec,
		}
		hzRestore.Spec.Persistence.Restore = restoreConfig
		return hzRestore
	}

	UserCodeBucket = func(lk types.NamespacedName, s, bkt string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{1}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				DeprecatedUserCodeDeployment: &hazelcastcomv1alpha1.UserCodeDeploymentConfig{
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
				},
			},
		}
	}

	JetConfigured = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},

			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(int32(1)),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               ptr.To(true),
					ResourceUploadEnabled: true,
				},
			},
		}
	}

	JetWithBucketConfigured = func(lk types.NamespacedName, s, bkt string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(int32(1)),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               ptr.To(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
				},
			},
		}
	}

	JetWithUrlConfigured = func(lk types.NamespacedName, url string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(int32(1)),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               ptr.To(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						RemoteURLs: []string{url},
					},
				},
			},
		}
	}

	JetWithLosslessRestart = func(lk types.NamespacedName, s, bkt string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(int32(1)),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               ptr.To(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
					Instance: &hazelcastcomv1alpha1.JetInstance{
						LosslessRestartEnabled:         true,
						CooperativeThreadCount:         ptr.To(int32(1)),
						MaxProcessorAccumulatedRecords: ptr.To(int64(1000000000)),
					},
				},
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   resource.NewQuantity(9*2^20, resource.BinarySI),
						StorageClassName: StorageClass,
					},
				},
			},
		}
	}

	JetWithRestore = func(lk types.NamespacedName, hbn string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(int32(1)),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               ptr.To(true),
					ResourceUploadEnabled: true,
					Instance: &hazelcastcomv1alpha1.JetInstance{
						LosslessRestartEnabled:         true,
						CooperativeThreadCount:         ptr.To(int32(1)),
						MaxProcessorAccumulatedRecords: ptr.To(int64(1000000000)),
					},
				},
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   resource.NewQuantity(9*2^20, resource.BinarySI),
						StorageClassName: StorageClass,
					},
					Restore: hazelcastcomv1alpha1.RestoreConfiguration{
						HotBackupResourceName: hbn,
					},
				},
			},
		}
	}

	UserCodeURL = func(lk types.NamespacedName, urls []string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{1}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				DeprecatedUserCodeDeployment: &hazelcastcomv1alpha1.UserCodeDeploymentConfig{
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						RemoteURLs: urls,
					},
				},
			},
		}
	}

	ExecutorService = func(lk types.NamespacedName, allExecutorServices map[string]interface{}, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				LoggingLevel:              hazelcastcomv1alpha1.LoggingLevelDebug,
				ClusterSize:               &[]int32{1}[0],
				Repository:                *hazelcastEERepo,
				Version:                   *hazelcastVersion,
				LicenseKeySecretName:      naming.LicenseKeySecret,
				ExecutorServices:          allExecutorServices["es"].([]hazelcastcomv1alpha1.ExecutorServiceConfiguration),
				DurableExecutorServices:   allExecutorServices["des"].([]hazelcastcomv1alpha1.DurableExecutorServiceConfiguration),
				ScheduledExecutorServices: allExecutorServices["ses"].([]hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration),
			},
		}
	}

	HighAvailability = func(lk types.NamespacedName, size int32, mode hazelcastcomv1alpha1.HighAvailabilityMode, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &size,
				HighAvailabilityMode: mode,
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	HazelcastTLS = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				TLS: &hazelcastcomv1alpha1.TLS{
					SecretName: lk.Name + "-tls",
				},
			},
		}
	}

	HazelcastMTLS = func(lk types.NamespacedName, lbls map[string]string, isOptional bool) *hazelcastcomv1alpha1.Hazelcast {
		ma := hazelcastcomv1alpha1.MutualAuthenticationRequired
		if isOptional {
			ma = hazelcastcomv1alpha1.MutualAuthenticationOptional
		}
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				TLS: &hazelcastcomv1alpha1.TLS{
					SecretName:           lk.Name + "-mtls",
					MutualAuthentication: ma,
				},
			},
		}
	}

	HazelcastSQLPersistence = func(lk types.NamespacedName, clusterSize int32, labels map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(clusterSize),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
						StorageClassName: StorageClass,
					},
				},
				SQL: &hazelcastcomv1alpha1.SQL{
					CatalogPersistenceEnabled: true,
				},
			},
		}
	}

	HotBackupBucket = func(lk types.NamespacedName, hzName string, lbls map[string]string, bucketURI, secretName string) *hazelcastcomv1alpha1.HotBackup {
		return &hazelcastcomv1alpha1.HotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HotBackupSpec{
				HazelcastResourceName: hzName,
				BucketURI:             bucketURI,
				SecretName:            secretName,
			},
		}
	}

	HotBackup = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.HotBackup {
		return &hazelcastcomv1alpha1.HotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HotBackupSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	CronHotBackup = func(lk types.NamespacedName, schedule string, hbSpec *hazelcastcomv1alpha1.HotBackupSpec, lbls map[string]string) *hazelcastcomv1alpha1.CronHotBackup {
		return &hazelcastcomv1alpha1.CronHotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.CronHotBackupSpec{
				Schedule: schedule,
				HotBackupTemplate: hazelcastcomv1alpha1.HotBackupTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Labels: lbls,
					},
					Spec: *hbSpec,
				},
			},
		}
	}

	Faulty = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          &[]int32{3}[0],
				Repository:           *hazelcastEERepo,
				LicenseKeySecretName: naming.LicenseKeySecret,
				Version:              "not-exists",
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			},
		}
	}

	DefaultMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	PersistedMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
					BackupCount:           ptr.To(int32(0)),
				},
				PersistenceEnabled: true,
			},
		}
	}

	Map = func(ms hazelcastcomv1alpha1.MapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: ms,
		}
	}

	BackupCountMap = func(lk types.NamespacedName, hzName string, lbls map[string]string, backupCount int32) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
					BackupCount:           &backupCount,
				},
			},
		}
	}

	MapWithEventJournal = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
				EventJournal: &hazelcastcomv1alpha1.EventJournal{},
			},
		}
	}

	DefaultTieredStoreMap = func(lk types.NamespacedName, hzName string, deviceName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
				InMemoryFormat: hazelcastcomv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastcomv1alpha1.TieredStore{
					DiskDeviceName: deviceName,
				},
			},
		}
	}

	DefaultWanReplication = func(wan types.NamespacedName, mapName, targetClusterName, endpoints string, lbls map[string]string) *hazelcastcomv1alpha1.WanReplication {
		return &hazelcastcomv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources: []hazelcastcomv1alpha1.ResourceSpec{{
					Name: mapName,
					Kind: hazelcastcomv1alpha1.ResourceKindMap,
				}},
			},
		}
	}

	CustomWanReplication = func(wan types.NamespacedName, targetClusterName, endpoints string, lbls map[string]string) *hazelcastcomv1alpha1.WanReplication {
		return &hazelcastcomv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources:         []hazelcastcomv1alpha1.ResourceSpec{},
			},
		}
	}

	WanReplication = func(wan types.NamespacedName, targetClusterName, endpoints string, resources []hazelcastcomv1alpha1.ResourceSpec, lbls map[string]string) *hazelcastcomv1alpha1.WanReplication {
		return &hazelcastcomv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources:         resources,
			},
		}
	}

	WanSync = func(wan types.NamespacedName, wanReplicationName string, lbls map[string]string) *hazelcastcomv1alpha1.WanSync {
		return &hazelcastcomv1alpha1.WanSync{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanSyncSpec{
				WanReplicationResourceName: wanReplicationName,
			},
		}
	}

	DefaultMultiMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.MultiMap {
		return &hazelcastcomv1alpha1.MultiMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MultiMapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	DefaultTopic = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Topic {
		return &hazelcastcomv1alpha1.Topic{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.TopicSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	DefaultReplicatedMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.ReplicatedMap {
		return &hazelcastcomv1alpha1.ReplicatedMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ReplicatedMapSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	DefaultQueue = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Queue {
		return &hazelcastcomv1alpha1.Queue{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.QueueSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	DefaultCache = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Cache {
		return &hazelcastcomv1alpha1.Cache{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.CacheSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
				InMemoryFormat: hazelcastcomv1alpha1.InMemoryFormatBinary,
			},
		}
	}

	MultiMap = func(mms hazelcastcomv1alpha1.MultiMapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.MultiMap {
		return &hazelcastcomv1alpha1.MultiMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: mms,
		}
	}

	Topic = func(mms hazelcastcomv1alpha1.TopicSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Topic {
		return &hazelcastcomv1alpha1.Topic{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: mms,
		}
	}

	ReplicatedMap = func(rms hazelcastcomv1alpha1.ReplicatedMapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ReplicatedMap {
		return &hazelcastcomv1alpha1.ReplicatedMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: rms,
		}
	}

	Queue = func(qs hazelcastcomv1alpha1.QueueSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Queue {
		return &hazelcastcomv1alpha1.Queue{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: qs,
		}
	}

	Cache = func(cs hazelcastcomv1alpha1.CacheSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Cache {
		return &hazelcastcomv1alpha1.Cache{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: cs,
		}
	}

	JetJob = func(jarName string, hz string, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.JetJob {
		return &hazelcastcomv1alpha1.JetJob{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.JetJobSpec{
				Name:                  lk.Name,
				HazelcastResourceName: hz,
				State:                 hazelcastcomv1alpha1.RunningJobState,
				JarName:               jarName,
			},
		}
	}

	JetJobWithInitialSnapshot = func(jarName string, hz string, snapshotResourceName string, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.JetJob {
		return &hazelcastcomv1alpha1.JetJob{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.JetJobSpec{
				Name:                        lk.Name,
				HazelcastResourceName:       hz,
				State:                       hazelcastcomv1alpha1.RunningJobState,
				JarName:                     jarName,
				InitialSnapshotResourceName: snapshotResourceName,
			},
		}
	}

	JetJobSnapshot = func(name string, cancel bool, jetJobResourceName string, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.JetJobSnapshot {
		return &hazelcastcomv1alpha1.JetJobSnapshot{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.JetJobSnapshotSpec{
				Name:               name,
				CancelJob:          cancel,
				JetJobResourceName: jetJobResourceName,
			},
		}
	}

	TLSSecret = func(lk types.NamespacedName, lbls map[string]string, useCertChain bool) *corev1.Secret {
		var data map[string][]byte
		if useCertChain {
			data = map[string][]byte{
				corev1.TLSCertKey:       []byte(ServerCert),
				corev1.TLSPrivateKeyKey: []byte(PrivKey),
				"ca.crt":                []byte(RootCACert),
			}
		} else {
			data = map[string][]byte{
				corev1.TLSCertKey:       []byte(ExampleCert),
				corev1.TLSPrivateKeyKey: []byte(ExampleKey),
			}
		}
		return &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Data: data,
		}
	}

	HazelcastTieredStorage = func(lk types.NamespacedName, deviceName string, labels map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				Agent: hazelcastcomv1alpha1.AgentConfiguration{
					Repository: *agentRepo,
					Version:    *agentVersion,
				},
				ClusterSize:          ptr.To(int32(3)),
				Repository:           *hazelcastEERepo,
				Version:              *hazelcastVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				NativeMemory: &hazelcastcomv1alpha1.NativeMemoryConfiguration{
					AllocatorType: hazelcastcomv1alpha1.NativeMemoryStandard,
				},
				LocalDevices: []hazelcastcomv1alpha1.LocalDeviceConfig{
					{
						Name: deviceName,
						PVC: &hazelcastcomv1alpha1.PvcConfiguration{
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: StorageClass,
						},
					},
				},
			},
		}
	}

	UserCodeNamespace = func(ucns hazelcastcomv1alpha1.UserCodeNamespaceSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.UserCodeNamespace {
		return &hazelcastcomv1alpha1.UserCodeNamespace{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: ucns,
		}
	}

	VectorCollection = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.VectorCollection {
		return &hazelcastcomv1alpha1.VectorCollection{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: hzName,
				Indexes: []hazelcastcomv1alpha1.VectorIndex{
					{
						Dimension:        10,
						EfConstruction:   256,
						MaxDegree:        32,
						Metric:           hazelcastcomv1alpha1.Dot,
						UseDeduplication: false,
					},
				},
			},
		}
	}
)

const (
	ExampleCert = `-----BEGIN CERTIFICATE-----
MIIDJDCCAgygAwIBAgIUKzKxkelznHkzTRJcAff41uYNOzwwDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHZXhhbXBsZTAeFw0yMzA1MDQxMjI1NTVaFw0zMzA1MDEx
MjI1NTVaMBIxEDAOBgNVBAMMB2V4YW1wbGUwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCtdKO42gtHph8X+Q5jIBVAuOfR9nGWZoaLuF5+741CTihygmqr
WBAxkxVmpIgD+kHsX04hC4ku4uyBEncRjWAtncH+3/AYYZ3/QC+l1/PMX5WfiJ8X
lEYxuOb0d86ZjgVdWVgRi3qyePHmdzBnPTFOfF5lc5SDdIWIbJ37/y0Ar5Wivftx
QMqzfLdK9cAdW3yd/D3tzfVlIHk1NarVJVxnwpfvvtAoGj+JkQ/ZGu9qrpgpLPOH
vsY0AuL1gEaNgHYTCZfkWsDklobceBaHB3boAKf8k91Xou9J6rAMHT8+clFgPPuV
jk5ws4eOgIgBFOL4zztqVaR9ZSsSvGfLWrSTAgMBAAGjcjBwMB0GA1UdDgQWBBQX
eKHFreouZ5JhhocAynjN94i2tjAfBgNVHSMEGDAWgBQXeKHFreouZ5JhhocAynjN
94i2tjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEF
BQcDAjANBgkqhkiG9w0BAQsFAAOCAQEAXFHpdAZmxMAWZX2P65c2kNdSUu8dfKmp
GO0HbInAY/nnaKVPwwKs3J58DMQON7a4RXyMS8s3l6DlVwdxbGrBdc74fCFgGtRX
R4e5B6O4kGYedFx1GlFlbShzWSCu3RUjMPZ7bQlqELtXGh9Zz7sE0MZqJsTLgnDd
E8m2YHdnzHmvbwprs9z4J8vsbUZL/zWheWCwergogEKA9sqUf82jUHKlLPELDykb
/tZkWIEmH3HZ3iIzv1W0aq+TcjfL5Pm+OBG36KgyLg0jJJ6G3rj+NqWyeGZYcN0P
j7p1jMEkX90CJIweXgzJvPJ1UcpP7amZCHk4N2adz8QVRee4DRECmw==
-----END CERTIFICATE-----
	`

	ExampleKey = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCtdKO42gtHph8X
+Q5jIBVAuOfR9nGWZoaLuF5+741CTihygmqrWBAxkxVmpIgD+kHsX04hC4ku4uyB
EncRjWAtncH+3/AYYZ3/QC+l1/PMX5WfiJ8XlEYxuOb0d86ZjgVdWVgRi3qyePHm
dzBnPTFOfF5lc5SDdIWIbJ37/y0Ar5WivftxQMqzfLdK9cAdW3yd/D3tzfVlIHk1
NarVJVxnwpfvvtAoGj+JkQ/ZGu9qrpgpLPOHvsY0AuL1gEaNgHYTCZfkWsDklobc
eBaHB3boAKf8k91Xou9J6rAMHT8+clFgPPuVjk5ws4eOgIgBFOL4zztqVaR9ZSsS
vGfLWrSTAgMBAAECggEAOHAXRXJM8UcwHtC+yaoKwEBpzXtugg1iAdw/gvXW9JgR
uRCOPKouurKs5/To/MJU6OApv77NKCBV67liXKevf6gxEwkySfyZOBBecIvPm9QO
DxaZDUcFf/A11Z2V74iyXilP6oWDqsaHjwGBElZq0KrO3Bu7Wvpy6GzPCsuAjRQK
iELvY0E57RZvie5aHtZKdhV8ZbtC40dZzO9hboBZUv1kdEyf8bmcKYseB8WRjmxj
Z2BsqxS1reMzaVJX/qC3294Cpyv6G4zioJ0AKUijUZk+HhK5dB14asA1xfC394DR
gUDthuEZtRIAA/DuMhuyG+n2xoEWoagN7uU0Lei3YQKBgQC76sH0ygWqAKZxPWLV
Co7NxC0GN4U+FbWQDQ0ohGHqNV8BBS7Ja5OaQyr9HF8AEYiAWDa2jZYDBQkGYJNK
F3IvOdmez/DQT3/qPUbqxfVoJQGAnLmGrEPYzCM9Wns8OQsVZmcG0TfZrd7u5tDm
rcOBF7m5BXF5NPIBCxD7+fvBTwKBgQDsTJeFsMeffomsMHZkpfqfn6gZRq/0K512
xHPTWjPE8emG22AAnAuLi7/s9UJ5UAyrAOOOtwJicCFFdA7r1Y+dkTq/WCPQUWQL
JfGIpbyrSIlQS+n/w+xProNMx4xtbtkUek+PWVC8C9NuRvXb+HzKbPflaGYBFm2A
rCbk7/9ffQKBgDIGykHHsoBSkfzdkb0ThXbj/fSEvVUM5HwH7XPW4lY+hR85aP44
RGAx93TQo73Z7RP16ALraH8/TOrEtRFpcn1+EiBETWC3eV87lvCTaMSj7WV207E1
lQ5XMh54QwyCRyAYVd8rvYmWzx2clwqCQeTREyFdgJr67F44uvnJ0CrjAoGAYyZk
MdGSgYcL53dSRjsq5U2NsEVr0S133fziiN2BeXL0RQTJzJetdHlIJ/plURfYqOwv
j5OU6Y8ZNtZS6HvszfXBS8aFCIUOUGs0ZNz+RHSkQVAJOKuR/YFBULcuYkCvz5re
xUx5xt3DcrNNuGYUnq+IePcMTgqGGgaiL0/QvNUCgYB+BU9WvRfLtY8SPa8HEyQm
XNNtM+ABirkTi+Vym4Z1Y48ky1kBM5PuCFAbndlPvWSzcwy5jd+obB4mD4Q/wSVz
BJV1B7N4JQrVXBpOKW9TtJdHQ2WoTsx8AAnFEZ+C2BfNg+jqgvPtgZpwIcY+wcqG
nq1euTfl0SHp6nrtumTvwg==
-----END PRIVATE KEY-----
`

	ServerCert = `Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            74:00:f6:fe:24:01:7c:46:97:45:06:2f:f7:65:4d:a2
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: C=US, O=Hazelcast Test, CN=Hazelcast Test Sub CA
        Validity
            Not Before: Sep  3 18:05:07 2024 GMT
            Not After : Aug 29 18:05:07 2044 GMT
        Subject: C=US, O=Hazelcast Test, CN=server
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (2048 bit)
                Modulus:
                    00:c2:87:5a:5f:a1:f8:7a:3b:0a:30:e5:95:a1:0b:
                    6b:8b:8b:2a:c8:92:a1:12:a0:55:10:7c:0b:20:e7:
                    7e:f3:15:96:ed:ce:fb:87:8e:3b:b9:74:a3:aa:c8:
                    13:70:de:9e:8c:28:3e:fb:ce:16:78:63:4f:f3:6d:
                    b1:90:2e:fd:78:b5:5f:75:45:ce:97:b3:f6:e1:27:
                    1a:65:8f:f1:38:2c:31:e8:d6:49:d5:54:a4:e3:6f:
                    d2:49:72:41:a5:d1:33:18:79:7e:a5:90:09:ff:25:
                    0c:16:10:d5:e3:e8:a4:eb:ff:38:9d:3e:18:77:90:
                    fc:13:d4:69:01:cb:08:46:2a:44:c8:9a:9e:89:12:
                    97:59:b6:35:b6:63:b1:2f:b6:a1:5f:57:28:65:4e:
                    c5:5f:f9:6b:71:5a:9a:4a:a8:04:54:db:13:b9:f1:
                    bc:83:2b:4c:b3:7e:c6:69:06:35:04:2e:4a:e8:2a:
                    d3:1e:8e:38:46:48:5f:c1:2a:73:ef:7c:9b:aa:94:
                    30:ca:1f:99:9d:b7:ca:c6:e3:bc:78:bb:28:98:09:
                    1f:3c:bd:c0:65:e1:d3:f7:a1:75:32:af:14:94:d9:
                    b6:aa:61:5d:64:e4:4d:6f:c7:a0:72:ac:b3:28:95:
                    1f:13:60:b4:d2:fb:6c:03:88:67:30:f0:28:67:ca:
                    87:47
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            Authority Information Access:
                CA Issuers - URI:http://sub-ca.hazelcast-test.download/sub-ca.crt
                OCSP - URI:http://ocsp.sub-ca.hazelcast-test.download:9081
            X509v3 Subject Key Identifier:
                2C:4D:38:65:6A:B3:2E:0B:0C:F6:19:6E:60:12:41:E2:52:9C:4B:AD
            X509v3 Basic Constraints: critical
                CA:FALSE
            X509v3 CRL Distribution Points:
                Full Name:
                  URI:http://sub-ca.hazelcast-test.download/sub-ca.crl
            X509v3 Extended Key Usage:
                TLS Web Client Authentication, TLS Web Server Authentication
            X509v3 Key Usage: critical
                Digital Signature, Key Encipherment
            X509v3 Authority Key Identifier:
                27:73:67:BD:56:C8:73:9B:DC:A4:A2:77:10:88:0B:DB:59:AB:C7:F1
    Signature Algorithm: sha256WithRSAEncryption
    Signature Value:
        56:0e:f7:8e:63:07:40:17:d7:74:67:ac:96:8b:8d:3d:16:68:
        e5:e0:b9:19:ba:5b:94:46:46:2e:63:f7:bb:aa:31:20:b2:92:
        f5:cb:f0:7d:e8:37:ab:80:55:93:b9:76:72:71:06:22:92:27:
        bf:38:ea:2a:ce:01:85:16:b8:7f:a5:a9:c0:1f:e6:d6:9a:3d:
        2e:d5:1a:b8:0b:9b:6d:cc:f0:a3:93:86:d7:dd:eb:63:16:39:
        b9:60:85:31:9a:04:47:9b:41:6e:8f:19:53:b2:e0:b8:de:4d:
        c2:9c:70:2a:b4:ba:80:22:f0:9e:d5:5a:bd:b5:72:8c:24:bb:
        3e:c9:a6:f6:91:7a:d1:08:dd:bc:d1:e8:97:29:d1:f6:b9:12:
        d2:14:65:14:e4:87:72:4b:e1:6a:3d:b9:0a:72:c0:51:af:14:
        17:ca:aa:5f:78:e2:9b:c1:d4:2e:63:58:b6:fa:07:a0:53:45:
        db:32:70:79:65:8e:3f:9e:4e:55:b7:49:27:4f:2a:7d:d6:2f:
        4b:f4:86:46:fc:e0:a3:0a:e8:d7:b7:41:21:8d:71:c3:6a:e6:
        93:96:b1:a8:6e:84:cd:3d:7a:ed:a0:4a:e2:28:69:3d:03:fa:
        d6:dd:9e:7b:f3:b5:37:5d:ac:e1:6d:32:a6:52:53:d1:55:82:
        f8:1d:92:db
-----BEGIN CERTIFICATE-----
MIIEWTCCA0GgAwIBAgIQdAD2/iQBfEaXRQYv92VNojANBgkqhkiG9w0BAQsFADBG
MQswCQYDVQQGEwJVUzEXMBUGA1UECgwOSGF6ZWxjYXN0IFRlc3QxHjAcBgNVBAMM
FUhhemVsY2FzdCBUZXN0IFN1YiBDQTAeFw0yNDA5MDMxODA1MDdaFw00NDA4Mjkx
ODA1MDdaMDcxCzAJBgNVBAYTAlVTMRcwFQYDVQQKDA5IYXplbGNhc3QgVGVzdDEP
MA0GA1UEAwwGc2VydmVyMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
wodaX6H4ejsKMOWVoQtri4sqyJKhEqBVEHwLIOd+8xWW7c77h447uXSjqsgTcN6e
jCg++84WeGNP822xkC79eLVfdUXOl7P24ScaZY/xOCwx6NZJ1VSk42/SSXJBpdEz
GHl+pZAJ/yUMFhDV4+ik6/84nT4Yd5D8E9RpAcsIRipEyJqeiRKXWbY1tmOxL7ah
X1coZU7FX/lrcVqaSqgEVNsTufG8gytMs37GaQY1BC5K6CrTHo44RkhfwSpz73yb
qpQwyh+ZnbfKxuO8eLsomAkfPL3AZeHT96F1Mq8UlNm2qmFdZORNb8egcqyzKJUf
E2C00vtsA4hnMPAoZ8qHRwIDAQABo4IBUDCCAUwwgYkGCCsGAQUFBwEBBH0wezA8
BggrBgEFBQcwAoYwaHR0cDovL3N1Yi1jYS5oYXplbGNhc3QtdGVzdC5kb3dubG9h
ZC9zdWItY2EuY3J0MDsGCCsGAQUFBzABhi9odHRwOi8vb2NzcC5zdWItY2EuaGF6
ZWxjYXN0LXRlc3QuZG93bmxvYWQ6OTA4MTAdBgNVHQ4EFgQULE04ZWqzLgsM9hlu
YBJB4lKcS60wDAYDVR0TAQH/BAIwADBBBgNVHR8EOjA4MDagNKAyhjBodHRwOi8v
c3ViLWNhLmhhemVsY2FzdC10ZXN0LmRvd25sb2FkL3N1Yi1jYS5jcmwwHQYDVR0l
BBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMA4GA1UdDwEB/wQEAwIFoDAfBgNVHSME
GDAWgBQnc2e9Vshzm9ykoncQiAvbWavH8TANBgkqhkiG9w0BAQsFAAOCAQEAVg73
jmMHQBfXdGeslouNPRZo5eC5GbpblEZGLmP3u6oxILKS9cvwfeg3q4BVk7l2cnEG
IpInvzjqKs4BhRa4f6WpwB/m1po9LtUauAubbczwo5OG193rYxY5uWCFMZoER5tB
bo8ZU7LguN5NwpxwKrS6gCLwntVavbVyjCS7Psmm9pF60QjdvNHolynR9rkS0hRl
FOSHckvhaj25CnLAUa8UF8qqX3jim8HULmNYtvoHoFNF2zJweWWOP55OVbdJJ08q
fdYvS/SGRvzgowro17dBIY1xw2rmk5axqG6EzT167aBK4ihpPQP61t2ee/O1N12s
4W0yplJT0VWC+B2S2w==
-----END CERTIFICATE-----
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            99:d1:f2:64:92:6e:22:d9:2e:7f:b2:ce:7c:6d:39:85
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: C=US, O=Hazelcast Test, CN=Hazelcast Test Root CA
        Validity
            Not Before: Sep  3 18:05:07 2024 GMT
            Not After : Aug 29 18:05:07 2044 GMT
        Subject: C=US, O=Hazelcast Test, CN=Hazelcast Test Sub CA
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (2048 bit)
                Modulus:
                    00:cb:61:01:df:1c:57:7f:37:ad:4c:0c:90:c5:21:
                    4d:01:70:3b:2c:9a:6d:0c:54:ab:7a:cc:8b:d1:e8:
                    df:aa:73:16:e7:45:bb:e7:65:92:1e:89:30:b2:4f:
                    29:3f:43:6c:d3:cc:33:62:8d:33:7f:88:66:a5:10:
                    e0:e0:f2:f5:06:81:08:40:65:98:5e:25:fe:82:2f:
                    51:f1:a2:00:5e:b2:fe:79:7e:2d:03:a3:d1:81:01:
                    80:30:b2:e5:f2:fc:79:13:1f:91:ba:e8:86:db:24:
                    00:b6:b8:bc:36:6d:10:28:af:70:ee:5a:af:a2:fc:
                    e2:e5:46:dd:03:63:1b:18:70:09:71:a3:c1:c5:47:
                    70:bd:2c:c3:b9:a2:cd:40:e2:60:d9:b6:8a:47:47:
                    99:ed:60:4c:4e:19:96:ea:61:60:ef:d5:e7:60:b5:
                    bb:b2:81:16:03:bb:ab:31:6e:70:5f:43:fc:60:3a:
                    88:b1:65:d8:93:62:6b:7a:58:25:ae:c2:fd:62:cc:
                    0b:a7:13:0b:43:c9:c0:d5:64:63:72:74:90:29:4c:
                    ed:28:1d:4b:56:f3:d4:b0:29:5b:9a:e7:c9:35:f0:
                    87:e3:47:16:c4:e3:7a:0a:68:c3:a5:10:41:98:76:
                    c5:8e:bd:65:54:67:88:3b:e8:a9:42:be:5b:00:a8:
                    63:e3
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            Authority Information Access:
                CA Issuers - URI:http://root-ca.hazelcast-test.download/root-ca.crt
                OCSP - URI:http://ocsp.root-ca.hazelcast-test.download:9080
            X509v3 Subject Key Identifier:
                27:73:67:BD:56:C8:73:9B:DC:A4:A2:77:10:88:0B:DB:59:AB:C7:F1
            X509v3 Basic Constraints: critical
                CA:TRUE, pathlen:0
            X509v3 CRL Distribution Points:
                Full Name:
                  URI:http://root-ca.hazelcast-test.download/root-ca.crl
            X509v3 Extended Key Usage:
                TLS Web Client Authentication, TLS Web Server Authentication
            X509v3 Key Usage: critical
                Certificate Sign, CRL Sign
            X509v3 Authority Key Identifier:
                00:1E:06:19:9E:CC:B8:EF:B3:4F:48:BA:F4:4D:07:A3:6E:61:B6:1F
    Signature Algorithm: sha256WithRSAEncryption
    Signature Value:
        26:e1:ed:da:55:25:37:68:0c:fc:c3:75:ec:02:4b:a2:9e:e5:
        d0:93:a7:f8:cb:c8:2a:b2:48:60:f5:80:b6:39:76:81:c2:e9:
        9f:c6:ff:bb:86:fb:09:da:ec:5e:6a:ee:9b:48:e3:54:5b:98:
        98:2f:b6:b4:ec:78:20:49:ed:30:46:9c:27:36:e2:42:b5:f4:
        2e:a9:c3:f5:2e:07:12:21:a9:40:bd:ad:3c:b1:4d:95:99:a2:
        88:71:16:8b:ec:5d:af:d2:e3:50:a9:c9:e4:39:03:9c:39:4b:
        1f:63:66:82:47:69:bf:03:0e:77:a3:ff:6a:ec:6e:cb:c3:7b:
        02:bd:5c:fe:f8:88:2b:04:67:88:f1:2a:10:9a:df:d5:bd:62:
        e0:f3:18:b5:0b:70:ea:43:6d:88:87:97:56:d3:14:e9:0b:3e:
        f5:38:f0:ed:50:69:8e:1f:ac:bc:59:52:ec:2a:84:2f:5e:5d:
        c0:fc:00:cc:b5:1b:51:79:69:18:d1:0f:f2:5c:df:e1:c5:f7:
        e7:e7:33:cf:82:56:47:58:aa:5d:d2:10:3b:6d:04:24:95:01:
        85:26:60:ff:41:5d:b6:0f:c3:45:2c:53:fd:47:e1:e2:55:05:
        51:27:07:8e:7b:6a:a3:cd:ae:4a:0d:1d:c6:e3:71:c7:12:e1:
        2c:c4:f3:2c:78:b4:ed:b8:ea:37:83:93:b4:a6:6c:1b:5a:36:
        e3:ae:8e:f8:7e:73:24:1c:7f:bd:bd:11:ae:7c:81:7f:62:dd:
        7f:f0:f0:f2:e6:03:ad:56:28:d1:4c:c4:dc:fa:8b:f3:0d:62:
        87:a7:80:a6:ae:cd:32:38:97:77:c3:b7:81:61:74:ec:6f:e0:
        f3:94:f9:64:2d:de:39:56:e8:55:99:19:6d:a3:ce:34:ff:98:
        14:f9:ac:d2:4c:49:4a:e6:1f:59:4b:9c:f6:e9:8c:84:57:32:
        6e:e3:3d:f6:92:a9:8e:f4:6e:24:0f:6f:a8:0d:83:76:4a:53:
        12:29:0c:83:0f:a2:30:f5:cb:05:cf:b8:90:c2:b4:ac:a3:fd:
        e0:6b:3d:73:b4:fd:27:a3:0a:aa:8e:66:e4:7b:03:13:ba:ba:
        6d:49:27:ff:d7:b6:ae:8e:36:0f:ba:33:eb:9e:9c:a2:fd:f6:
        c1:e4:fe:11:53:0a:28:ca:54:20:07:db:4a:42:51:c3:9a:16:
        96:48:c5:70:65:73:81:46:f5:85:69:03:3b:9d:10:98:94:f6:
        f8:30:73:f1:2a:22:e6:cf:12:42:93:bf:5d:17:7c:45:ec:1c:
        4c:50:2f:fb:a9:2a:60:af:c9:5e:d2:64:10:35:d2:73:85:1a:
        00:a9:ea:bf:ef:79:59:f4
-----BEGIN CERTIFICATE-----
MIIFdjCCA16gAwIBAgIRAJnR8mSSbiLZLn+yznxtOYUwDQYJKoZIhvcNAQELBQAw
RzELMAkGA1UEBhMCVVMxFzAVBgNVBAoMDkhhemVsY2FzdCBUZXN0MR8wHQYDVQQD
DBZIYXplbGNhc3QgVGVzdCBSb290IENBMB4XDTI0MDkwMzE4MDUwN1oXDTQ0MDgy
OTE4MDUwN1owRjELMAkGA1UEBhMCVVMxFzAVBgNVBAoMDkhhemVsY2FzdCBUZXN0
MR4wHAYDVQQDDBVIYXplbGNhc3QgVGVzdCBTdWIgQ0EwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDLYQHfHFd/N61MDJDFIU0BcDssmm0MVKt6zIvR6N+q
cxbnRbvnZZIeiTCyTyk/Q2zTzDNijTN/iGalEODg8vUGgQhAZZheJf6CL1HxogBe
sv55fi0Do9GBAYAwsuXy/HkTH5G66IbbJAC2uLw2bRAor3DuWq+i/OLlRt0DYxsY
cAlxo8HFR3C9LMO5os1A4mDZtopHR5ntYExOGZbqYWDv1edgtbuygRYDu6sxbnBf
Q/xgOoixZdiTYmt6WCWuwv1izAunEwtDycDVZGNydJApTO0oHUtW89SwKVua58k1
8IfjRxbE43oKaMOlEEGYdsWOvWVUZ4g76KlCvlsAqGPjAgMBAAGjggFcMIIBWDCB
jQYIKwYBBQUHAQEEgYAwfjA+BggrBgEFBQcwAoYyaHR0cDovL3Jvb3QtY2EuaGF6
ZWxjYXN0LXRlc3QuZG93bmxvYWQvcm9vdC1jYS5jcnQwPAYIKwYBBQUHMAGGMGh0
dHA6Ly9vY3NwLnJvb3QtY2EuaGF6ZWxjYXN0LXRlc3QuZG93bmxvYWQ6OTA4MDAd
BgNVHQ4EFgQUJ3NnvVbIc5vcpKJ3EIgL21mrx/EwEgYDVR0TAQH/BAgwBgEB/wIB
ADBDBgNVHR8EPDA6MDigNqA0hjJodHRwOi8vcm9vdC1jYS5oYXplbGNhc3QtdGVz
dC5kb3dubG9hZC9yb290LWNhLmNybDAdBgNVHSUEFjAUBggrBgEFBQcDAgYIKwYB
BQUHAwEwDgYDVR0PAQH/BAQDAgEGMB8GA1UdIwQYMBaAFAAeBhmezLjvs09IuvRN
B6NuYbYfMA0GCSqGSIb3DQEBCwUAA4ICAQAm4e3aVSU3aAz8w3XsAkuinuXQk6f4
y8gqskhg9YC2OXaBwumfxv+7hvsJ2uxeau6bSONUW5iYL7a07HggSe0wRpwnNuJC
tfQuqcP1LgcSIalAva08sU2VmaKIcRaL7F2v0uNQqcnkOQOcOUsfY2aCR2m/Aw53
o/9q7G7Lw3sCvVz++IgrBGeI8SoQmt/VvWLg8xi1C3DqQ22Ih5dW0xTpCz71OPDt
UGmOH6y8WVLsKoQvXl3A/ADMtRtReWkY0Q/yXN/hxffn5zPPglZHWKpd0hA7bQQk
lQGFJmD/QV22D8NFLFP9R+HiVQVRJweOe2qjza5KDR3G43HHEuEsxPMseLTtuOo3
g5O0pmwbWjbjro74fnMkHH+9vRGufIF/Yt1/8PDy5gOtVijRTMTc+ovzDWKHp4Cm
rs0yOJd3w7eBYXTsb+DzlPlkLd45VuhVmRlto840/5gU+azSTElK5h9ZS5z26YyE
VzJu4z32kqmO9G4kD2+oDYN2SlMSKQyDD6Iw9csFz7iQwrSso/3gaz1ztP0nowqq
jmbkewMTurptSSf/17aujjYPujPrnpyi/fbB5P4RUwooylQgB9tKQlHDmhaWSMVw
ZXOBRvWFaQM7nRCYlPb4MHPxKiLmzxJCk79dF3xF7BxMUC/7qSpgr8le0mQQNdJz
hRoAqeq/73lZ9A==
-----END CERTIFICATE-----
`

	PrivKey = `-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQDCh1pfofh6Owow
5ZWhC2uLiyrIkqESoFUQfAsg537zFZbtzvuHjju5dKOqyBNw3p6MKD77zhZ4Y0/z
bbGQLv14tV91Rc6Xs/bhJxplj/E4LDHo1knVVKTjb9JJckGl0TMYeX6lkAn/JQwW
ENXj6KTr/zidPhh3kPwT1GkBywhGKkTImp6JEpdZtjW2Y7EvtqFfVyhlTsVf+Wtx
WppKqARU2xO58byDK0yzfsZpBjUELkroKtMejjhGSF/BKnPvfJuqlDDKH5mdt8rG
47x4uyiYCR88vcBl4dP3oXUyrxSU2baqYV1k5E1vx6ByrLMolR8TYLTS+2wDiGcw
8ChnyodHAgMBAAECggEABLA1beOd9PgyTS5jVk/Lpj/S5qWeCzBhDHYo4ICjzyEA
k7eu2TwE1XnpreaHjWtYH+GibvgvE3S1SxUkN+jiBAQ/CjkF+yMDurZyDOuUsTlj
dIyhl+oj1TVvOITv7xqlJBxdgIkBrKwMaAW82fLT8roid6u09EDCyomOhFQL3YEB
h+HwvcZpyx4Ygv3izeYMxkGvT32rqh6lc/Ka0ws2vE0H8EhR7D8cDg3ZhZ+uU2jn
LnJNL8su3GgnbilNdkW+jlCsQtHWCdB4CLbByguV0Whg2e8oQnT4mZjg8oBdvXZr
vczJZzvzep+eqE2sbPRR7DVZbQZ+OIdLb5khM6ZJwQKBgQDkf2IC4dyHl+KRwwPH
Jf1FzjT37opTeEd76j24fGl9SK20kdQnp0dMZ/99lcj6MmEYD+IlNp77hXmuv+vo
XjO8KZgWN9sCLAJghwT8kbEl9Ch5QWg4qs093f9YGa1xEvoyGb2bnOVYkKht9f4a
uCCdSPKj4pkAQ6ZoZd8a80hOwQKBgQDZ8UyauhFrz8iEaaBKq6mG9CtBNdvW/mqc
NIaDDDt73KVFAvVFrHpcAPUaEgLPL5tYY9WqFO6QXr+lCoamN4q0OEGfUDS7KF4p
TuVk7bRf/1aEYZphn9BshDO7d0jATb1JXwooDFW2yL1B5ZKuYULZtnNP2FRHw9nl
sFA2vUhgBwKBgQDBkcQXCv3GhH302480w1MHMsQekR7vzUJJkEuPIR5AezRkdvGS
UhyNdsCyxBRJGCq2tqXuvpH6I73Ms1uHM16CdX4YvGK1OVEeMuOfj1DSBT/QUP+Y
meFbGti46q/KzbfUf4fn7wc/evSkirMkMX23oNekzE6vMaAkasCRVS2ZQQKBgQCS
pAknEosmP2hrr6Zql5Y5d5CjD9objpOtBqp7AoADlzKcfKELgEHUJdDE+dlqDl43
2vSou+zItve71JlEvZpWKIP+7biNNVwl7y/p+QakkOllqUZ26VETsuAcAuawfZ4f
ABOVXrdNhUPSUuWe71JLqrdrweLzZpP2N+vA6RsJgQKBgQCh16aPkPGVjDf3pEKe
1oIrYIIFSVMBUDQvjzkSssv2O3FAnPNKAaGlyO3VrX0Dhh5HhUsP2a9qpbdxw0Oc
jdgN3Ri52aKgwBMaAFD+sfhgVtEzDD803wUsn5k0VptzlSwxCbt3p698l3TdNiIl
1uvIzt4vhmbE1AAdx4ryWQFtsw==
-----END PRIVATE KEY-----
`

	RootCACert = `Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            99:d1:f2:64:92:6e:22:d9:2e:7f:b2:ce:7c:6d:39:84
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: C=US, O=Hazelcast Test, CN=Hazelcast Test Root CA
        Validity
            Not Before: Sep  3 18:05:07 2024 GMT
            Not After : Aug 29 18:05:07 2044 GMT
        Subject: C=US, O=Hazelcast Test, CN=Hazelcast Test Root CA
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (4096 bit)
                Modulus:
                    00:a2:e8:d3:00:e4:84:df:95:64:13:52:a0:62:6c:
                    37:ee:7f:66:51:b1:5d:96:02:df:6a:32:b7:aa:fc:
                    a9:3e:cf:52:0f:39:24:8e:e2:08:bb:d1:c0:0d:d9:
                    43:dc:7f:53:9b:1a:2b:60:c8:10:b0:fd:af:43:92:
                    7e:61:0c:97:72:1f:ea:7a:b7:ad:d4:cd:b5:4c:ad:
                    20:34:c8:39:d5:d2:91:95:8e:14:61:99:67:4c:81:
                    e8:35:cf:06:2b:da:16:dc:e1:94:59:86:8b:a3:76:
                    bb:f6:61:8e:50:c4:5a:42:14:20:59:88:b5:f0:d7:
                    34:50:04:1c:97:1c:bd:a5:43:06:10:59:f7:64:61:
                    1b:01:2a:88:8e:ab:94:3b:eb:c1:f9:af:bb:40:2f:
                    45:c5:cc:fd:3d:7b:be:ca:32:2a:8f:f4:85:ba:c0:
                    67:05:be:dc:0d:c6:73:51:26:ca:f6:94:ed:39:1a:
                    52:0e:01:08:d1:1a:7a:b7:9d:21:db:7c:1d:2b:b9:
                    1c:51:6c:0a:f5:b2:12:84:e3:cc:b7:9b:42:d3:13:
                    85:af:7e:58:9d:e7:9c:1b:2e:58:3f:e8:2b:06:94:
                    9c:72:3c:3c:50:b7:5a:cc:2c:e0:81:1c:06:65:fc:
                    94:7d:9f:91:f9:c0:83:a4:bb:55:3e:ef:17:6d:9f:
                    ba:2f:07:2b:56:98:55:66:24:9d:d6:eb:1d:74:94:
                    e8:75:63:4c:71:ea:26:64:30:30:c9:36:8d:e0:40:
                    6b:b8:43:f1:51:e8:6d:e7:77:3c:86:31:36:1d:92:
                    f6:6a:2f:d5:38:ce:c7:57:e3:8e:f1:3d:e4:4c:b6:
                    8d:2c:fc:51:d1:3b:d3:e0:0a:c3:b5:9a:30:6a:63:
                    6a:86:0e:b2:69:f1:8b:f5:22:c5:d2:96:9f:e7:e5:
                    b3:f2:2b:20:fd:a7:8d:ec:31:79:91:ab:91:10:e5:
                    c8:88:bb:53:5a:b3:56:7c:9f:5b:32:fa:99:0a:a6:
                    0b:a5:82:c5:14:67:2b:9b:47:a9:d4:5e:4e:3a:34:
                    ca:bd:e7:40:78:51:b6:69:c7:f7:39:3b:0d:8c:6b:
                    22:5b:8f:ce:e1:26:e5:9e:ad:a6:41:db:58:c7:b9:
                    34:87:47:73:8c:f3:6f:ad:8d:fd:c3:20:3f:cf:e6:
                    4d:ab:70:ed:1d:22:8b:e0:08:f3:85:6a:70:33:fa:
                    48:96:68:6d:60:6f:56:86:69:fc:dc:81:40:22:13:
                    3f:f8:8b:a4:e0:30:9f:c1:12:68:d4:ed:14:8c:df:
                    5a:3e:c2:f3:bd:71:dd:6f:e2:92:39:b7:d1:36:5a:
                    db:0c:44:9f:58:d2:de:b2:84:64:2b:a9:b0:ad:8c:
                    44:e6:33
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Basic Constraints: critical
                CA:TRUE
            X509v3 Key Usage: critical
                Certificate Sign, CRL Sign
            X509v3 Subject Key Identifier:
                00:1E:06:19:9E:CC:B8:EF:B3:4F:48:BA:F4:4D:07:A3:6E:61:B6:1F
    Signature Algorithm: sha256WithRSAEncryption
    Signature Value:
        4c:3b:0f:a0:29:2d:8d:52:87:95:ce:ed:79:2c:07:28:9c:f7:
        a8:21:c3:48:91:48:5d:64:bf:73:df:0c:d8:4f:ea:6e:eb:03:
        b8:a5:84:75:10:df:ce:d0:f4:aa:b0:3d:1b:8f:ef:80:59:e4:
        0d:24:d1:79:b9:3b:f0:41:aa:af:09:21:84:2b:0d:07:4e:ad:
        20:4e:a0:1b:93:cf:c5:60:08:b3:ee:32:66:19:73:be:3f:6a:
        60:34:b6:85:48:a8:75:6d:58:2b:d1:b9:6d:84:da:c8:a4:9e:
        fe:d2:f6:ac:b2:dc:12:90:fd:45:6a:fb:33:dd:d0:1b:f0:6e:
        90:87:42:51:8a:6f:0e:da:b0:36:fb:b2:1a:d9:8b:cc:fd:b3:
        57:d4:40:07:ca:2c:63:c6:ef:ed:1c:47:a6:77:8d:1e:b9:e5:
        78:92:8c:00:3a:10:71:95:e7:d2:dc:a2:88:0a:ff:45:74:6a:
        76:e2:ff:ce:22:3f:55:f5:53:3b:18:d4:72:29:59:19:d0:91:
        22:79:81:56:13:75:ee:55:b6:57:00:3a:8e:1f:07:77:d3:c4:
        9f:4f:ab:98:55:f7:ef:95:85:c4:35:da:3f:d5:33:42:c1:f7:
        94:1d:7c:3d:19:47:97:ad:9a:9a:37:11:0c:5b:2c:78:58:3d:
        14:d4:ea:3b:f3:1f:eb:7b:ed:3e:57:1d:f2:17:02:fe:2f:10:
        fb:d8:ed:f3:4e:85:cf:d5:04:0d:3d:29:1e:65:c5:20:44:3b:
        34:36:0a:35:62:33:61:74:05:4b:e7:76:80:52:27:26:5b:24:
        b2:3e:e7:2c:9f:ae:9c:39:62:3e:a6:b9:86:65:10:39:d9:c9:
        8c:12:a1:3c:4f:c8:73:f4:78:dd:04:b2:61:09:aa:3b:22:ce:
        7e:1c:bc:9d:e7:87:b8:71:2c:38:b5:2c:27:29:e8:d8:f1:c3:
        c9:36:31:3e:42:dd:78:63:b6:fd:b4:da:0f:a4:57:a5:6e:db:
        10:f5:15:2a:71:f3:6c:7b:d2:9e:37:c7:52:d1:06:76:90:3d:
        be:85:2d:87:f4:68:d5:32:1a:1b:66:2a:92:5d:f9:b8:d9:a1:
        36:c4:5b:e5:a1:1a:99:87:79:bf:67:cc:6c:8c:fb:2f:46:6b:
        29:21:ba:41:5a:cd:56:d0:1e:f3:d8:dc:24:1c:0c:c8:a0:43:
        63:7b:16:76:55:63:b2:26:a7:4a:c9:6b:46:68:6f:9b:6c:a3:
        ee:1a:7f:5a:bd:be:e4:da:af:ae:4f:81:89:a8:bc:36:18:e1:
        e5:2d:88:a5:e4:18:5e:f3:80:00:2b:45:8d:8e:2c:30:c0:71:
        92:18:ae:fb:7c:3b:4d:37
-----BEGIN CERTIFICATE-----
MIIFWzCCA0OgAwIBAgIRAJnR8mSSbiLZLn+yznxtOYQwDQYJKoZIhvcNAQELBQAw
RzELMAkGA1UEBhMCVVMxFzAVBgNVBAoMDkhhemVsY2FzdCBUZXN0MR8wHQYDVQQD
DBZIYXplbGNhc3QgVGVzdCBSb290IENBMB4XDTI0MDkwMzE4MDUwN1oXDTQ0MDgy
OTE4MDUwN1owRzELMAkGA1UEBhMCVVMxFzAVBgNVBAoMDkhhemVsY2FzdCBUZXN0
MR8wHQYDVQQDDBZIYXplbGNhc3QgVGVzdCBSb290IENBMIICIjANBgkqhkiG9w0B
AQEFAAOCAg8AMIICCgKCAgEAoujTAOSE35VkE1KgYmw37n9mUbFdlgLfajK3qvyp
Ps9SDzkkjuIIu9HADdlD3H9TmxorYMgQsP2vQ5J+YQyXch/qeret1M21TK0gNMg5
1dKRlY4UYZlnTIHoNc8GK9oW3OGUWYaLo3a79mGOUMRaQhQgWYi18Nc0UAQclxy9
pUMGEFn3ZGEbASqIjquUO+vB+a+7QC9Fxcz9PXu+yjIqj/SFusBnBb7cDcZzUSbK
9pTtORpSDgEI0Rp6t50h23wdK7kcUWwK9bIShOPMt5tC0xOFr35YneecGy5YP+gr
BpSccjw8ULdazCzggRwGZfyUfZ+R+cCDpLtVPu8XbZ+6LwcrVphVZiSd1usddJTo
dWNMceomZDAwyTaN4EBruEPxUeht53c8hjE2HZL2ai/VOM7HV+OO8T3kTLaNLPxR
0TvT4ArDtZowamNqhg6yafGL9SLF0paf5+Wz8isg/aeN7DF5kauREOXIiLtTWrNW
fJ9bMvqZCqYLpYLFFGcrm0ep1F5OOjTKvedAeFG2acf3OTsNjGsiW4/O4Sblnq2m
QdtYx7k0h0dzjPNvrY39wyA/z+ZNq3DtHSKL4AjzhWpwM/pIlmhtYG9Whmn83IFA
IhM/+Iuk4DCfwRJo1O0UjN9aPsLzvXHdb+KSObfRNlrbDESfWNLesoRkK6mwrYxE
5jMCAwEAAaNCMEAwDwYDVR0TAQH/BAUwAwEB/zAOBgNVHQ8BAf8EBAMCAQYwHQYD
VR0OBBYEFAAeBhmezLjvs09IuvRNB6NuYbYfMA0GCSqGSIb3DQEBCwUAA4ICAQBM
Ow+gKS2NUoeVzu15LAconPeoIcNIkUhdZL9z3wzYT+pu6wO4pYR1EN/O0PSqsD0b
j++AWeQNJNF5uTvwQaqvCSGEKw0HTq0gTqAbk8/FYAiz7jJmGXO+P2pgNLaFSKh1
bVgr0blthNrIpJ7+0vasstwSkP1Favsz3dAb8G6Qh0JRim8O2rA2+7Ia2YvM/bNX
1EAHyixjxu/tHEemd40eueV4kowAOhBxlefS3KKICv9FdGp24v/OIj9V9VM7GNRy
KVkZ0JEieYFWE3XuVbZXADqOHwd308SfT6uYVffvlYXENdo/1TNCwfeUHXw9GUeX
rZqaNxEMWyx4WD0U1Oo78x/re+0+Vx3yFwL+LxD72O3zToXP1QQNPSkeZcUgRDs0
Ngo1YjNhdAVL53aAUicmWySyPucsn66cOWI+prmGZRA52cmMEqE8T8hz9HjdBLJh
Cao7Is5+HLyd54e4cSw4tSwnKejY8cPJNjE+Qt14Y7b9tNoPpFelbtsQ9RUqcfNs
e9KeN8dS0QZ2kD2+hS2H9GjVMhobZiqSXfm42aE2xFvloRqZh3m/Z8xsjPsvRmsp
IbpBWs1W0B7z2NwkHAzIoENjexZ2VWOyJqdKyWtGaG+bbKPuGn9avb7k2q+uT4GJ
qLw2GOHlLYil5Bhe84AAK0WNjiwwwHGSGK77fDtNNw==
-----END CERTIFICATE-----
`
)
