package v1alpha1

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type datastructValidator struct {
	fieldValidator
}

func NewDatastructValidator(o client.Object) datastructValidator {
	return datastructValidator{NewFieldValidator(o)}
}

func (v *datastructValidator) validateDSSpecUnchanged(obj client.Object) {
	ok, err := isDSSpecUnchanged(obj)
	if err != nil {
		v.InternalError(Path("spec"), err)
		return
	}
	if !ok {
		v.Forbidden(Path("spec"), "cannot be updated")
	}
}

func isDSSpecUnchanged(obj client.Object) (bool, error) {
	lastSpec, ok := obj.GetAnnotations()[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return true, nil
	}
	ds, ok := obj.(DataStructure)
	if !ok {
		return false, errors.New("object is not a data structure")
	}
	newSpec, err := ds.GetSpec()
	if err != nil {
		return false, errors.New("could not get spec of the data structure")
	}
	return newSpec == lastSpec, nil
}
