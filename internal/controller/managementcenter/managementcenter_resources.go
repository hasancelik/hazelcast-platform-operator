package managementcenter

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	"github.com/hazelcast/hazelcast-platform-operator/internal/tls"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// Environment variables used for Management Center configuration
const (
	// mcLicenseKey License key for Management Center
	mcLicenseKey = "MC_LICENSE_KEY"
	// mcInitCmd init command for Management Center
	mcInitCmd = "MC_INIT_CMD"
	// javaOpts java options for Management Center
	javaOpts = "JAVA_OPTS"
)

func (r *ManagementCenterReconciler) executeFinalizer(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter) error {
	if !controllerutil.ContainsFinalizer(mc, n.Finalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(mc, n.Finalizer)
	err := r.Update(ctx, mc)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *ManagementCenterReconciler) reconcileService(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	service := &corev1.Service{
		ObjectMeta: controller.Metadata(mc, n.ManagementCenter),
		Spec: corev1.ServiceSpec{
			Selector: controller.SelectorLabels(mc, n.ManagementCenter),
		},
	}

	err := controllerutil.SetControllerReference(mc, service, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Service: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, service, func() error {
		service.Spec.Type = mc.Spec.ExternalConnectivity.ManagementCenterServiceType()
		mcPorts := []corev1.ServicePort{controller.HTTPPort(8080, n.MancenterPort), controller.HTTPSPort(443, n.MancenterPort)}
		service.Spec.Ports = util.EnrichServiceNodePorts(mcPorts, service.Spec.Ports)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Service", mc.Name, "result", opResult)
	}
	return err
}

func (r *ManagementCenterReconciler) reconcileIngress(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	ingress := &networkingv1.Ingress{
		ObjectMeta: controller.Metadata(mc, n.ManagementCenter),
		Spec:       networkingv1.IngressSpec{},
	}

	if !mc.Spec.ExternalConnectivity.Ingress.IsEnabled() {
		err := r.Client.Delete(ctx, ingress)
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			logger.Info("Deleting ingress", "Ingress", mc.Name)
		}
		return nil
	}

	err := controllerutil.SetControllerReference(mc, ingress, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Ingress: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, ingress, func() error {
		ingress.Spec.IngressClassName = &mc.Spec.ExternalConnectivity.Ingress.IngressClassName
		ingress.ObjectMeta.Annotations = mc.Spec.ExternalConnectivity.Ingress.Annotations
		ingress.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: mc.Spec.ExternalConnectivity.Ingress.Hostname,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     mc.Spec.ExternalConnectivity.Ingress.Path,
								PathType: &[]networkingv1.PathType{networkingv1.PathTypePrefix}[0],
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: controller.Metadata(mc, n.ManagementCenter).Name,
										Port: networkingv1.ServiceBackendPort{
											Name: controller.HTTPPort(8080, n.MancenterPort).Name,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Ingress", mc.Name, "result", opResult)
	}
	return err
}

func (r *ManagementCenterReconciler) reconcileRoute(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	if platform.GetType() != platform.OpenShift {
		return nil
	}

	route := &routev1.Route{
		ObjectMeta: controller.Metadata(mc, n.ManagementCenter),
		Spec:       routev1.RouteSpec{},
	}

	if !mc.Spec.ExternalConnectivity.Route.IsEnabled() {
		err := r.Client.Delete(ctx, route)
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			logger.Info("Deleting route", "Route", mc.Name)
		}
		return nil
	}

	err := controllerutil.SetControllerReference(mc, route, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Route: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, route, func() error {
		route.Spec = routev1.RouteSpec{
			Host: mc.Spec.ExternalConnectivity.Route.Hostname,
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: controller.Metadata(mc, n.ManagementCenter).Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("http"),
			},
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Route", mc.Name, "result", opResult)
	}
	return err
}

func (r *ManagementCenterReconciler) reconcileStatefulset(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: controller.Metadata(mc, n.ManagementCenter),
		Spec: appsv1.StatefulSetSpec{
			// Management Center StatefulSet size is always 1
			Replicas: &[]int32{1}[0],
			Selector: &metav1.LabelSelector{
				MatchLabels: controller.SelectorLabels(mc, n.ManagementCenter),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: controller.Labels(mc, n.ManagementCenter),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: n.ManagementCenter,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Name:          n.MancenterPort,
							Protocol:      corev1.ProtocolTCP,
						}},
						VolumeMounts: []corev1.VolumeMount{},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Port:   intstr.FromInt32(8081),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt32(8080),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						SecurityContext: controller.ContainerSecurityContext(),
					}},
					SecurityContext: controller.PodSecurityContext(),
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(mc, sts, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on StatefulSet: %w", err)
	}

	if mc.Spec.Persistence.IsEnabled() {
		if mc.Spec.Persistence.ExistingVolumeClaimName == "" {
			sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{persistentVolumeClaim(mc)}
		} else {
			sts.Spec.Template.Spec.Volumes = []corev1.Volume{existingVolumeClaim(mc.Spec.Persistence.ExistingVolumeClaimName)}
		}
	} else {
		// Add emptyDir volume to make /data writable
		sts.Spec.Template.Spec.Volumes = []corev1.Volume{emptyDirVolume(mc.Spec.Persistence.ExistingVolumeClaimName)}
	}

	sts.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{persistentVolumeMount(), tmpDirMount(), configMount()}

	// Add tmpDir to make /tmp writable
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, tmpDir())

	// Mount client configs generated by Hazelcast reconciler
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, configVolume(mc))

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, sts, func() error {
		sts.Spec.Template.Spec.ImagePullSecrets = mc.Spec.ImagePullSecrets
		sts.Spec.Template.Spec.Containers[0].Image = mc.DockerImage()
		sts.Spec.Template.Spec.Containers[0].Env = env(ctx, mc, r.Client, logger)
		sts.Spec.Template.Spec.Containers[0].ImagePullPolicy = mc.Spec.ImagePullPolicy
		sts.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Path = path.Join(getRootPath(mc), "health")
		if mc.Spec.Resources != nil {
			sts.Spec.Template.Spec.Containers[0].Resources = *mc.Spec.Resources
		}

		if mc.Spec.Scheduling != nil {
			sts.Spec.Template.Spec.Affinity = mc.Spec.Scheduling.Affinity
			sts.Spec.Template.Spec.Tolerations = mc.Spec.Scheduling.Tolerations
			sts.Spec.Template.Spec.NodeSelector = mc.Spec.Scheduling.NodeSelector
			sts.Spec.Template.Spec.TopologySpreadConstraints = mc.Spec.Scheduling.TopologySpreadConstraints
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", mc.Name, "result", opResult)
	}
	return err
}

func getRootPath(mc *hazelcastv1alpha1.ManagementCenter) string {
	if mc.Spec.ExternalConnectivity.IsEnabled() && mc.Spec.ExternalConnectivity.Ingress != nil {
		return mc.Spec.ExternalConnectivity.Ingress.Path
	}
	return "/"
}

func (r *ManagementCenterReconciler) reconcileSecret(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	secret := &corev1.Secret{
		ObjectMeta: controller.Metadata(mc, n.ManagementCenter),
	}

	err := controllerutil.SetControllerReference(mc, secret, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Secret: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, secret, func() error {
		files := make(map[string][]byte)
		for _, clusterConf := range mc.Spec.HazelcastClusters {
			clientConfig, err := hazelcastClientConfig(&clusterConf)
			if err != nil {
				return err
			}
			files[clusterConf.Name+".yaml"] = clientConfig

			if clusterConf.TLS == nil {
				continue
			}
			keystore, truststore, err := tls.HazelcastKeyAndTrustStore(ctx, r.Client, clusterConf.TLS.SecretName, mc.Namespace)
			if err != nil {
				return err
			}
			files[clusterConf.Name+"-keystore.p12"] = keystore
			files[clusterConf.Name+"-truststore.p12"] = truststore
		}
		secret.Data = files
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Secret", secret.Name, "result", opResult)
	}
	return err
}

func persistentVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.MancenterStorageName,
		MountPath: "/data",
	}
}

func tmpDirMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.TmpDirVolName,
		MountPath: "/tmp",
	}
}

func persistentVolumeClaim(mc *hazelcastv1alpha1.ManagementCenter) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        n.MancenterStorageName,
			Namespace:   mc.Namespace,
			Labels:      controller.Labels(mc, n.ManagementCenter),
			Annotations: mc.Spec.Annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: mc.Spec.Persistence.StorageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *mc.Spec.Persistence.Size,
				},
			},
		},
	}
}

func existingVolumeClaim(claimName string) corev1.Volume {
	return corev1.Volume{
		Name: n.MancenterStorageName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
			},
		},
	}
}

func emptyDirVolume(claimName string) corev1.Volume {
	return corev1.Volume{
		Name: n.MancenterStorageName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func tmpDir() corev1.Volume {
	return corev1.Volume{
		Name: n.TmpDirVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func configVolume(mc *hazelcastv1alpha1.ManagementCenter) corev1.Volume {
	var defaultMode int32 = 420
	return corev1.Volume{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  mc.Name,
				DefaultMode: &defaultMode,
			},
		},
	}
}

func configMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "config",
		MountPath: "/config",
	}
}

func env(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, c client.Client, logger logr.Logger) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  mcInitCmd,
			Value: buildMcInitCmd(ctx, mc, c, logger),
		},
	}

	if mc.Spec.GetLicenseKeySecretName() != "" {
		envs = append(envs,
			corev1.EnvVar{
				Name: mcLicenseKey,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: mc.Spec.GetLicenseKeySecretName(),
						},
						Key: n.LicenseDataKey,
					},
				},
			},
		)
	}

	// This env must be set after MC_LICENSE_KEY env var since it might have a reference
	// to MC_LICENSE_KEY (e.g. -Dhazelcast.mc.license=$(MC_LICENSE_KEY)).
	envs = append(envs,
		corev1.EnvVar{
			Name:  javaOpts,
			Value: javaOPTS(mc),
		},
	)

	envs = append(envs, mc.Spec.Env...)

	return envs
}

func buildMcInitCmd(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, c client.Client, logger logr.Logger) string {
	var commands []string
	if addCluster := clusterAddCommand(mc); addCluster != "" {
		commands = append(commands, addCluster)
	}
	if mc.Spec.SecurityProviders.IsEnabled() && !mc.Status.Configured {
		commands = append(commands, ldapConfigure(ctx, mc, c, logger)...)
	}
	return strings.Join(commands, " && ")
}

func ldapConfigure(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, c client.Client, logger logr.Logger) []string {
	ldap := mc.Spec.SecurityProviders.LDAP
	s := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: ldap.CredentialsSecretName, Namespace: mc.Namespace}, s)
	if err != nil {
		logger.Error(err, "unable to get the secret with credentials, LDAP config will be ignored")
	}
	return []string{"./bin/hz-mc conf security reset -H /data",
		fmt.Sprintf("./bin/hz-mc conf ldap configure -H /data --url=%q --ldap-username=%q "+
			"--ldap-password=%q --user-dn=%q --group-dn=%q --user-search-filter=%q --group-search-filter=%q "+
			"--admin-groups=%q --read-write-groups=%q --read-only-groups=%q --metrics-only-groups=%q",
			ldap.URL, string(s.Data["username"]), string(s.Data["password"]), ldap.UserDN, ldap.GroupDN,
			ldap.UserSearchFilter, ldap.GroupSearchFilter, strings.Join(ldap.AdminGroups, ","),
			strings.Join(ldap.UserGroups, ","), strings.Join(ldap.ReadonlyUserGroups, ","),
			strings.Join(ldap.MetricsOnlyGroups, ","))}
}

func clusterAddCommand(mc *hazelcastv1alpha1.ManagementCenter) string {
	var commands []string
	for _, cluster := range mc.Spec.HazelcastClusters {
		commands = append(commands, fmt.Sprintf("./bin/mc-conf.sh cluster add --lenient=true -H /data --client-config %s", path.Join("/config", cluster.Name+".yaml")))
	}
	return strings.Join(commands, " && ")
}

func javaOPTS(mc *hazelcastv1alpha1.ManagementCenter) string {
	args := []string{
		"-Dhazelcast.mc.healthCheck.enable=true",
		"-Dhazelcast.mc.lock.skip=true",
		"-Dhazelcast.mc.tls.enabled=false",
		"-Dmancenter.ssl=false",
		fmt.Sprintf("-Dhazelcast.mc.phone.home.enabled=%t", util.IsPhoneHomeEnabled()),
	}

	if mc.Spec.GetLicenseKeySecretName() != "" {
		args = append(args, fmt.Sprintf("-Dhazelcast.mc.license=$(%s)", mcLicenseKey))
	}

	if mc.Spec.ExternalConnectivity.IsEnabled() && mc.Spec.ExternalConnectivity.Ingress != nil {
		args = append(args, fmt.Sprintf("-Dhazelcast.mc.contextPath=%s", getRootPath(mc)))
	}

	if mc.Spec.JVM.IsConfigured() {
		args = append(args, mc.Spec.JVM.Args...)
	}

	return strings.Join(args, " ")
}

func hazelcastClientConfig(config *hazelcastv1alpha1.HazelcastClusterConfig) ([]byte, error) {
	clientConfig := HazelcastClientWrapper{HazelcastClient{
		ClusterName: config.Name,
		Network: Network{
			ClusterMembers: []string{
				config.Address,
			},
			SSL: SSL{
				Enabled:          false,
				FactoryClassName: "com.hazelcast.nio.ssl.BasicSSLContextFactory",
			},
		},
	}}

	if config.TLS != nil && config.TLS.SecretName != "" {
		clientConfig.HazelcastClient.Network.SSL = SSL{
			Enabled:          true,
			FactoryClassName: "com.hazelcast.nio.ssl.BasicSSLContextFactory",
			Properties: NewSSLProperties(
				path.Join("/config", config.Name+"-keystore.p12"),
				path.Join("/config", config.Name+"-truststore.p12"),
				config.TLS.MutualAuthentication,
			),
		}
	}

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	if err := enc.Encode(clientConfig); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type HazelcastClientWrapper struct {
	HazelcastClient HazelcastClient `yaml:"hazelcast-client"`
}

type HazelcastClient struct {
	ClusterName string  `yaml:"cluster-name"`
	Network     Network `yaml:"network"`
}

type Network struct {
	ClusterMembers []string `yaml:"cluster-members,omitempty"`
	SSL            SSL      `yaml:"ssl,omitempty,omitempty"`
}

type SSL struct {
	Enabled          bool              `yaml:"enabled"`
	FactoryClassName string            `yaml:"factory-class-name"`
	Properties       map[string]string `yaml:"properties"`
}

func NewSSLProperties(keystorePath, truststorePath string, auth hazelcastv1alpha1.MutualAuthentication) map[string]string {
	const pass = "hazelcast"
	const typ = "PKCS12"
	switch auth {
	case hazelcastv1alpha1.MutualAuthenticationRequired:
		return map[string]string{
			"keyStoreType":       typ,
			"protocol":           "TLSv1.2",
			"keyStore":           keystorePath,
			"keyStorePassword":   pass,
			"trustStore":         truststorePath,
			"trustStorePassword": pass,
		}
	default:
		return map[string]string{
			"keyStoreType":       typ,
			"protocol":           "TLSv1.2",
			"trustStore":         truststorePath,
			"trustStorePassword": pass,
		}
	}
}
