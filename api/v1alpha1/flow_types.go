package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FlowSpec defines the desired state of Flow
type FlowSpec struct {
	// Number of Flow instances.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:default:=3
	// +optional
	Size *int32 `json:"size,omitempty"`

	// Repository to pull the Flow image from.
	// +kubebuilder:default:="quay.io/hazelcast_cloud/hazelcast-flow-internal"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Version of Flow.
	// +kubebuilder:default:="next-v593"
	// +optional
	Version string `json:"version,omitempty"`

	// Pull policy for the Flow image
	// +kubebuilder:default:="IfNotPresent"
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Image pull secrets for the Flow image
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Name of the secret with Hazelcast Enterprise License Key.
	// +kubebuilder:validation:MinLength:=1
	// +required
	LicenseKeySecretName string `json:"licenseKeySecretName"`

	// Scheduling details
	// +optional
	Scheduling *SchedulingConfiguration `json:"scheduling,omitempty"`

	// Compute Resources required by the Hazelcast container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Configuration to expose Flow to outside.
	// +kubebuilder:default:={}
	// +optional
	ExternalConnectivity *FlowExternalConnectivityConfiguration `json:"externalConnectivity,omitempty"`

	// Configuration for Flow database
	// +required
	Database DatabaseConfig `json:"database"`

	// Env configuration of environment variables
	// +kubebuilder:validation:XValidation:message="Environment variable name cannot be empty.",rule="self.all(env, env.name != '')"
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Flow Kubernetes resource annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Flow Kubernetes resource labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// FlowExternalConnectivityConfiguration defines how to expose Flow pods.
type FlowExternalConnectivityConfiguration struct {
	// Ingress configuration of Flow
	// +optional
	Ingress *ExternalConnectivityIngress `json:"ingress,omitempty"`

	// OpenShift Route configuration of Flow
	// +optional
	Route *ExternalConnectivityRoute `json:"route,omitempty"`
}

type DatabaseConfig struct {
	// Host to database
	// +kubebuilder:validation:MinLength:=1
	// +required
	Host string `json:"host"`

	// Name of the secret that contains DB credentials
	// +kubebuilder:validation:MinLength:=1
	// +required
	SecretName string `json:"secretName"`
}

func (f *Flow) DockerImage() string {
	return fmt.Sprintf("%s:%s", f.Spec.Repository, f.Spec.Version)
}

// FlowPhase represents the current state of the cluster
// +kubebuilder:validation:Enum=Running;Failed;Pending;Terminating
type FlowPhase string

const (
	// FlowRunning phase is the state when the Flow is successfully started
	FlowRunning FlowPhase = "Running"
	// FlowFailed phase is the state of error during the Flow startup
	FlowFailed FlowPhase = "Failed"
	// FlowPending phase is the state of starting the cluster when the Flow is not started yet
	FlowPending FlowPhase = "Pending"
	// FlowTerminating phase is the state where deletion of Flow and dependent resources happen
	FlowTerminating FlowPhase = "Terminating"
)

// FlowStatus defines the observed state of Flow
type FlowStatus struct {
	// Phase of the Flow
	// +optional
	Phase FlowPhase `json:"phase,omitempty"`

	// Message about the Flow state
	// +optional
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current state of the Flow deployment"

// Flow is the Schema for the flows API
type Flow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlowSpec   `json:"spec,omitempty"`
	Status FlowStatus `json:"status,omitempty"`
}

func (f *Flow) GetSpecAnnotations() map[string]string {
	return f.Spec.Annotations
}

func (f *Flow) GetSpecLabels() map[string]string {
	return f.Spec.Labels
}

//+kubebuilder:object:root=true

// FlowList contains a list of Flow
type FlowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Flow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Flow{}, &FlowList{})
}
