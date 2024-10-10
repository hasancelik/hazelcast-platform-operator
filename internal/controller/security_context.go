package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
)

func ContainerSecurityContext() *corev1.SecurityContext {
	sec := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		Privileged:               ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	return sec
}

func PodSecurityContext() *corev1.PodSecurityContext {
	// Openshift assigns user and fsgroup ids itself
	if platform.GetType() == platform.OpenShift {
		return &corev1.PodSecurityContext{
			RunAsNonRoot: ptr.To(true),
		}
	}

	var u int64 = 65534
	return &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		// Do not have to give User and FSGroup IDs because MC image's default user is 1001 so kubelet
		// does not complain when RunAsNonRoot is true
		// To keep it consistent with Hazelcast, we are adding following
		RunAsUser: &u,
		FSGroup:   &u,
	}
}
