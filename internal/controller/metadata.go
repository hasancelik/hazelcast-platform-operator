package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type MetadataObject interface {
	client.Object
	hazelcastv1alpha1.SpecAnnotatorLabeler
}

func Metadata[T MetadataObject](obj T, appName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		Labels:      Labels(obj, appName),
		Annotations: obj.GetSpecAnnotations(),
	}
}

func SelectorLabels[T MetadataObject](obj T, appName string) map[string]string {
	return map[string]string{
		n.ApplicationNameLabel:         appName,
		n.ApplicationInstanceNameLabel: obj.GetName(),
		n.ApplicationManagedByLabel:    n.OperatorName,
	}
}

func Labels[T MetadataObject](obj T, appName string) map[string]string {
	l := make(map[string]string)

	// copy user labels
	for name, value := range obj.GetSpecLabels() {
		l[name] = value
	}

	// make sure we overwrite user labels
	l[n.ApplicationNameLabel] = appName
	l[n.ApplicationInstanceNameLabel] = obj.GetName()
	l[n.ApplicationManagedByLabel] = n.OperatorName

	return l
}

func HTTPPort(port int32, targetPort string) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       "http",
		Protocol:   corev1.ProtocolTCP,
		Port:       port,
		TargetPort: intstr.FromString(targetPort),
	}
}

func HTTPSPort(port int32, targetPort string) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       "https",
		Protocol:   corev1.ProtocolTCP,
		Port:       port,
		TargetPort: intstr.FromString(targetPort),
	}
}
