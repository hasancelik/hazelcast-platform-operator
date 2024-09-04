package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

type topicValidator struct {
	datastructValidator
}

func NewTopicValidator(o client.Object) topicValidator {
	return topicValidator{NewDatastructValidator(o)}
}

func validateTopicSpecUpdate(t *Topic) error {
	v := NewTopicValidator(t)
	v.validateDSSpecUnchanged(t)
	return v.Err()
}
