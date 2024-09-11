package managementcenter

import (
	"flag"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var (
	mcVersion = flag.String("mc-version", naming.MCVersion, "Default Management Center version used in e2e tests")
	mcRepo    = flag.String("mc-repo", naming.MCRepo, "Management Center repository used in e2e tests")
)
var (
	Default = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		flag.Parse()
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              *mcVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				ExternalConnectivity: &hazelcastcomv1alpha1.ExternalConnectivityConfiguration{
					Type: hazelcastcomv1alpha1.ExternalConnectivityTypeLoadBalancer,
				},
				Persistence: &hazelcastcomv1alpha1.MCPersistenceConfiguration{
					Enabled:      ptr.To(true),
					Size:         &[]resource.Quantity{resource.MustParse("10Gi")}[0],
					StorageClass: hazelcast.StorageClass,
				},
			},
		}
	}

	PersistenceDisabled = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              *mcVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				ExternalConnectivity: &hazelcastcomv1alpha1.ExternalConnectivityConfiguration{
					Type: hazelcastcomv1alpha1.ExternalConnectivityTypeLoadBalancer,
				},
				HazelcastClusters: []hazelcastcomv1alpha1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: &hazelcastcomv1alpha1.MCPersistenceConfiguration{
					Enabled: ptr.To(false),
				},
			},
		}
	}

	RouteEnabled = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              *mcVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				ExternalConnectivity: &hazelcastcomv1alpha1.ExternalConnectivityConfiguration{

					Type: hazelcastcomv1alpha1.ExternalConnectivityTypeClusterIP,
					Route: &hazelcastcomv1alpha1.ExternalConnectivityRoute{
						Hostname: "",
					},
				},
				HazelcastClusters: []hazelcastcomv1alpha1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: &hazelcastcomv1alpha1.MCPersistenceConfiguration{
					Enabled: ptr.To(false),
				},
			},
		}
	}

	Faulty = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              "not-exists",
				LicenseKeySecretName: naming.LicenseKeySecret,
				Persistence: &hazelcastcomv1alpha1.MCPersistenceConfiguration{
					StorageClass: hazelcast.StorageClass},
			},
		}
	}

	TLSEnabled = func(lk types.NamespacedName, lbls map[string]string, isOptional bool) *hazelcastcomv1alpha1.ManagementCenter {
		ma := hazelcastcomv1alpha1.MutualAuthenticationRequired
		if isOptional {
			ma = hazelcastcomv1alpha1.MutualAuthenticationOptional
		}
		mc := Default(lk, lbls)
		mc.Spec.HazelcastClusters = []hazelcastcomv1alpha1.HazelcastClusterConfig{
			{
				Name:    "dev",
				Address: "hazelcast",
				TLS: &hazelcastcomv1alpha1.TLS{
					SecretName:           lk.Name + "-mtls",
					MutualAuthentication: ma,
				},
			},
		}
		return mc
	}
)
