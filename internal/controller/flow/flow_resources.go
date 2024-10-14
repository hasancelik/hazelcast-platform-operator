package flow

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

const (
	mcLicenseKey = "MC_LICENSE"
)

func (r *FlowReconciler) executeFinalizer(ctx context.Context, flow *hazelcastv1alpha1.Flow) error {
	if !controllerutil.ContainsFinalizer(flow, n.Finalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(flow, n.Finalizer)
	err := r.Update(ctx, flow)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *FlowReconciler) reconcileService(ctx context.Context, flow *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	service := &corev1.Service{
		ObjectMeta: controller.Metadata(flow, n.FlowApplicationName),
		Spec: corev1.ServiceSpec{
			Selector: controller.SelectorLabels(flow, n.FlowApplicationName),
		},
	}

	err := controllerutil.SetControllerReference(flow, service, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Service: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, service, func() error {
		service.Spec.Type = corev1.ServiceTypeClusterIP
		flowPort := controller.HTTPPort(n.FlowPort, n.FlowPortName)
		flowPort.Name = n.FlowApplicationName
		service.Spec.Ports = []corev1.ServicePort{
			flowPort,
			{
				Name:        n.HazelcastPortName,
				Port:        n.ClientServerSocketPort,
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: ptr.To("tcp"),
				TargetPort:  intstr.FromString(n.HazelcastPortName),
			},
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Service", flow.Name, "result", opResult)
	}
	return err
}

func (r *FlowReconciler) reconcileIngress(ctx context.Context, flow *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	ingress := &networkingv1.Ingress{
		ObjectMeta: controller.Metadata(flow, n.FlowApplicationName),
		Spec:       networkingv1.IngressSpec{},
	}

	if !flow.Spec.ExternalConnectivity.Ingress.IsEnabled() {
		err := r.Client.Delete(ctx, ingress)
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			logger.Info("Deleting ingress", "Ingress", flow.Name)
		}
		return nil
	}

	err := controllerutil.SetControllerReference(flow, ingress, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Ingress: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, ingress, func() error {
		ingress.Spec.IngressClassName = &flow.Spec.ExternalConnectivity.Ingress.IngressClassName
		ingress.ObjectMeta.Annotations = flow.Spec.ExternalConnectivity.Ingress.Annotations
		ingress.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: flow.Spec.ExternalConnectivity.Ingress.Hostname,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     flow.Spec.ExternalConnectivity.Ingress.Path,
								PathType: &[]networkingv1.PathType{networkingv1.PathTypePrefix}[0],
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: controller.Metadata(flow, n.FlowApplicationName).Name,
										Port: networkingv1.ServiceBackendPort{
											Name: n.FlowApplicationName,
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
		logger.Info("Operation result", "Ingress", flow.Name, "result", opResult)
	}
	return err
}

func (r *FlowReconciler) reconcileSecret(ctx context.Context, flow *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	scrt := &corev1.Secret{
		ObjectMeta: controller.Metadata(flow, n.Hazelcast),
		Data:       make(map[string][]byte),
	}

	err := controllerutil.SetControllerReference(flow, scrt, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Secret: %w", err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, err := controllerutil.CreateOrUpdate(ctx, r.Client, scrt, func() error {
			cfg, err := r.hazelcastConfig(flow)
			if err != nil {
				return err
			}
			scrt.Data["hazelcast.yaml"] = cfg
			return nil
		})
		if result != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Secret", flow.Name, "result", result)
		}
		return err
	})
}

func (r *FlowReconciler) hazelcastConfig(flow *hazelcastv1alpha1.Flow) ([]byte, error) {
	cfg := config.Hazelcast{
		ClusterName: n.FlowApplicationName,
		Jet: config.Jet{
			Enabled: ptr.To(true),
		},
		Network: config.Network{
			Join: config.Join{
				Kubernetes: config.Kubernetes{
					Enabled:     ptr.To(true),
					ServiceName: flow.Name,
					Namespace:   flow.Namespace,
				},
			},
			RestAPI: config.RestAPI{
				Enabled: ptr.To(true),
			},
		},
	}
	return yaml.Marshal(config.HazelcastWrapper{Hazelcast: cfg})
}

func (r *FlowReconciler) reconcileStatefulSet(ctx context.Context, flow *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: controller.Metadata(flow, n.FlowApplicationName),
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         flow.Name,
			Replicas:            flow.Spec.Size,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: controller.SelectorLabels(flow, n.FlowApplicationName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: controller.Labels(flow, n.FlowApplicationName),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: flow.Name,
					Volumes: []corev1.Volume{
						{
							Name: "tmp-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "workspace-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "hazelcast-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: flow.Name,
								},
							},
						},
					},
					Containers: []corev1.Container{{
						Name: n.FlowApplicationName,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: n.FlowPort,
								Name:          n.FlowPortName,
								Protocol:      corev1.ProtocolTCP,
							},
							{
								Name:          n.HazelcastPortName,
								ContainerPort: n.ClientServerSocketPort,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "tmp-dir",
								MountPath: "/tmp",
							},
							{
								Name:      "workspace-dir",
								MountPath: "/workspace/gitProjects",
							},
							{
								Name:      "hazelcast-config",
								MountPath: "/data/hazelcast",
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/api/actuator/health",
									Port:   intstr.FromInt32(n.FlowPort),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 5,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/api/actuator/health",
									Port:   intstr.FromInt32(n.FlowPort),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 5,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
					}},
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(flow, sts, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on StatefulSet: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, sts, func() error {
		sts.Spec.Template.Spec.ImagePullSecrets = flow.Spec.ImagePullSecrets
		sts.Spec.Template.Spec.Containers[0].Image = flow.DockerImage()
		sts.Spec.Template.Spec.Containers[0].Env, err = r.env(ctx, flow)
		if err != nil {
			return nil
		}
		sts.Spec.Template.Spec.Containers[0].ImagePullPolicy = flow.Spec.ImagePullPolicy
		if flow.Spec.Resources != nil {
			sts.Spec.Template.Spec.Containers[0].Resources = *flow.Spec.Resources
		}
		if flow.Spec.Scheduling != nil {
			sts.Spec.Template.Spec.Affinity = flow.Spec.Scheduling.Affinity
			sts.Spec.Template.Spec.Tolerations = flow.Spec.Scheduling.Tolerations
			sts.Spec.Template.Spec.NodeSelector = flow.Spec.Scheduling.NodeSelector
			sts.Spec.Template.Spec.TopologySpreadConstraints = flow.Spec.Scheduling.TopologySpreadConstraints
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "StatefulSet", flow.Name, "result", opResult)
	}
	return err
}

func (r *FlowReconciler) env(ctx context.Context, flow *hazelcastv1alpha1.Flow) ([]corev1.EnvVar, error) {
	var envs []corev1.EnvVar
	options := []string{
		"--flow.hazelcast.configYamlPath=/data/hazelcast/hazelcast.yaml",
		"--flow.schema.server.clustered=true",
	}
	dbOptions, err := r.flowDBOptions(ctx, flow)
	if err != nil {
		return envs, err
	}
	options = append(options, dbOptions)

	for _, envVar := range flow.Spec.Env {
		if envVar.Name == "OPTIONS" {
			options = append(options, envVar.Value)
		} else {
			envs = append(envs, envVar)
		}
	}
	envs = append(envs, corev1.EnvVar{
		Name:  "OPTIONS",
		Value: strings.Join(options, " "),
	}, corev1.EnvVar{
		Name: mcLicenseKey,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: flow.Spec.LicenseKeySecretName,
				},
				Key: n.LicenseDataKey,
			},
		},
	})
	return envs, nil
}

func (r *FlowReconciler) flowDBOptions(ctx context.Context, flow *hazelcastv1alpha1.Flow) (string, error) {
	s := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: flow.Spec.Database.SecretName, Namespace: flow.Namespace}, s)
	if err != nil {
		return "", nil
	}
	dbName := "flow"
	if s.Data["database"] != nil {
		dbName = string(s.Data["database"])
	}
	return fmt.Sprintf("--flow.db.username=%s --flow.db.password=%s --flow.db.host=%s --flow.db.database=%s",
		string(s.Data["username"]), string(s.Data["password"]), flow.Spec.Database.Host, dbName), nil
}

func (r *FlowReconciler) reconcileServiceAccount(ctx context.Context, flow *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      flow.Name,
			Namespace: flow.Namespace,
			Labels:    controller.Labels(flow, n.Hazelcast),
		},
	}

	err := controllerutil.SetControllerReference(flow, serviceAccount, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on ServiceAccount: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, serviceAccount, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ServiceAccount", flow.Name, "result", opResult)
	}
	return err
}

func (r *FlowReconciler) reconcileRole(ctx context.Context, flow *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      flow.Name,
			Namespace: flow.Namespace,
			Labels:    controller.Labels(flow, n.FlowApplicationName),
		},
	}

	err := controllerutil.SetControllerReference(flow, role, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Role: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "pods", "services"},
				Verbs:     []string{"get", "list"},
			},
		}
		return nil
	})

	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Role", flow.Name, "result", opResult)
	}
	return err
}

func (r *FlowReconciler) reconcileRoleBinding(ctx context.Context, flow *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      flow.Name,
			Namespace: flow.Namespace,
			Labels:    controller.Labels(flow, n.Hazelcast),
		},
	}

	err := controllerutil.SetControllerReference(flow, rb, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, rb, func() error {
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      flow.Name,
				Namespace: flow.Namespace,
			},
		}
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     flow.Name,
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "RoleBinding", flow.Name, "result", opResult)
	}
	return err
}
