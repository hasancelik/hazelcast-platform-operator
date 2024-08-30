package v1alpha1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VectorCollectionSpec defines the desired state of VectorCollection
type VectorCollectionSpec struct {
	// Name of the data structure config to be created. If empty, CR name will be used.
	// It cannot be updated after the config is created successfully.
	// +kubebuilder:validation:Pattern=^[a-zA-Z0-9_\-*]*$
	// +optional
	Name string `json:"name,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource that this resource is
	// created for.
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// Information about indexes configuration
	// +kubebuilder:validation:MinItems:=1
	// +required
	Indexes []VectorIndex `json:"indexes"`
}

type VectorIndex struct {

	// Name is name of the vector index. Can include letters, numbers, and the symbols - and _.
	// Required for single-index vector collections. Optional for multi-index collection.
	// +kubebuilder:validation:Pattern=^[a-zA-Z0-9_\-]*$
	// +optional
	Name string `json:"name,omitempty"`

	// Dimension of the vector.
	// +required
	Dimension int32 `json:"dimension"`

	// Metric is used to calculate the distance between two vectors.
	// +required
	Metric VectorMetric `json:"metric"`

	// MaxDegree is used to calculate the maximum number of neighbors per node. The calculation used is max-degree * 2.
	// +required
	MaxDegree int32 `json:"maxDegree"`

	// EfConstruction is the size of the search queue to use when finding nearest neighbors.
	// +required
	EfConstruction int32 `json:"efConstruction"`

	// UseDeduplication specify whether to use vector deduplication.
	// +required
	UseDeduplication bool `json:"useDeduplication"`
}

// +kubebuilder:validation:Enum=Euclidean;Cosine;Dot
type VectorMetric string

const (
	// Euclidean distance. Score definition: 1 / (1 + squareDistance(v1, v2))
	Euclidean VectorMetric = "Euclidean"

	// Cosine of the angle between the vectors. Score definition: (1 + cos(v1, v2)) / 2
	Cosine VectorMetric = "Cosine"

	// Dot product of the vectors. Score definition: (1 + dotProduct(v1, v2)) / 2
	Dot VectorMetric = "Dot"
)

// VectorCollectionStatus defines the observed state of VectorCollection
type VectorCollectionStatus struct {
	DataStructureStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Vector Collection Config"
//+kubebuilder:printcolumn:name="Hazelcast-Resource",type="string",priority=1,JSONPath=".spec.hazelcastResourceName",description="Name of the Hazelcast resource that this resource is created for"
//+kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Vector Collection Config"
//+kubebuilder:resource:shortName=vc

// VectorCollection is the Schema for the vectorcollections API
type VectorCollection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   VectorCollectionSpec   `json:"spec,omitempty"`
	Status VectorCollectionStatus `json:"status,omitempty"`
}

func (v *VectorCollection) GetDSName() string {
	if v.Spec.Name != "" {
		return v.Spec.Name
	}
	return v.Name
}

func (v *VectorCollection) GetHZResourceName() string {
	return v.Spec.HazelcastResourceName
}

func (v *VectorCollection) GetStatus() *DataStructureStatus {
	return &v.Status.DataStructureStatus
}

func (v *VectorCollection) GetSpec() (string, error) {
	cs, err := json.Marshal(v.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", v.Kind, err)
	}
	return string(cs), nil
}

func (v *VectorCollection) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &v.Spec); err != nil {
		return err
	}
	return nil
}

func (v *VectorCollection) ValidateSpecCurrent(_ *Hazelcast) error {
	return ValidateVectorCollectionSpec(v)
}

func (v *VectorCollection) ValidateSpecUpdate() error {
	return ValidateVectorCollectionSpec(v)
}

//+kubebuilder:object:root=true

// VectorCollectionList contains a list of VectorCollection
type VectorCollectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VectorCollection `json:"items"`
}

func (vl *VectorCollectionList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(vl.Items))
	for _, item := range vl.Items {
		l = append(l, client.Object(&item))
	}
	return l
}

func init() {
	SchemeBuilder.Register(&VectorCollection{}, &VectorCollectionList{})
}
