package v1alpha1

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type vectorCollectionValidator struct {
	datastructValidator
}

func newVectorCollectionValidator(o client.Object) vectorCollectionValidator {
	return vectorCollectionValidator{NewDatastructValidator(o)}
}

func ValidateVectorCollectionSpec(c *VectorCollection) error {
	v := newVectorCollectionValidator(c)
	v.validateDSSpecUnchanged(c)
	v.validateIndexes(c)
	return v.Err()
}

func (v *vectorCollectionValidator) validateIndexes(c *VectorCollection) {
	if len(c.Spec.Indexes) == 1 {
		return
	}
	for i, index := range c.Spec.Indexes {
		if index.Name == "" {
			v.Required(Path("spec", fmt.Sprintf("indexes[%d]", i), "name"), "must be set for multi-index collection.")
		}
	}
}
