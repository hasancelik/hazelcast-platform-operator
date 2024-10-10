package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var flowlog = logf.Log.WithName("flow-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *Flow) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-flow,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=flows,verbs=create;update,versions=v1alpha1,name=vflow.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Flow{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Flow) ValidateCreate() (admission.Warnings, error) {
	flowlog.Info("validate create", "name", r.Name)
	return nil, ValidateFlowSpec(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Flow) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	flowlog.Info("validate update", "name", r.Name)
	return nil, ValidateFlowSpec(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Flow) ValidateDelete() (admission.Warnings, error) {
	flowlog.Info("validate delete", "name", r.Name)
	return nil, nil
}
