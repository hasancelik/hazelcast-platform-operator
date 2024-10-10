package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
)

type flowValidator struct {
	fieldValidator
}

func newFlowValidator(o client.Object) flowValidator {
	return flowValidator{NewFieldValidator(o)}
}

func ValidateFlowSpec(f *Flow) error {
	v := newFlowValidator(f)
	v.validateSpecCurrent(f)
	v.validateSpecUpdate(f)
	return v.Err()
}

var restrictedOptions = []string{"--flow.hazelcast.configYamlPath", "--flow.schema.server.clustered"}

func (v *flowValidator) validateSpecCurrent(f *Flow) {
	for _, envVar := range f.Spec.Env {
		if envVar.Name == "OPTIONS" {
			for _, option := range restrictedOptions {
				if strings.Contains(envVar.Value, option) {
					v.Forbidden(Path("spec", "env"), fmt.Sprintf("Environment variable OPTIONS must not contain '%s'", option))
				}
			}
			if strings.Contains(envVar.Value, "--flow.db.") {
				v.Forbidden(Path("spec", "env"), "Environment variable OPTIONS must not contain '--flow.db.' configs. Use 'spec.database' instead.")
			}
		}
	}

	var secret corev1.Secret
	secretName := types.NamespacedName{
		Name:      f.Spec.Database.SecretName,
		Namespace: f.Namespace,
	}
	err := kubeclient.Get(context.Background(), secretName, &secret)
	if kerrors.IsNotFound(err) {
		// we care only about not found error
		v.NotFound(Path("spec", "database", "secretName"), "Database Secret not found")
		return
	}
}

func (v *flowValidator) validateSpecUpdate(_ *Flow) {
}
