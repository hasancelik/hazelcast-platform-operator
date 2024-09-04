package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type multimapValidator struct {
	datastructValidator
}

func NewMultiMapValidator(o client.Object) multimapValidator {
	return multimapValidator{NewDatastructValidator(o)}
}

func validateMultiMapSpecUpdate(mm *MultiMap) error {
	v := NewMultiMapValidator(mm)
	v.validateDSSpecUnchanged(mm)
	return v.Err()
}
