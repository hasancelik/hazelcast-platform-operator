package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var vectorcollectionlog = logf.Log.WithName("vectorcollection-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *VectorCollection) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-vectorcollection,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=vectorcollections,verbs=create;update,versions=v1alpha1,name=vvectorcollection.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &VectorCollection{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *VectorCollection) ValidateCreate() (admission.Warnings, error) {
	vectorcollectionlog.Info("validate create", "name", r.Name)
	return admission.Warnings{}, ValidateVectorCollectionSpec(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *VectorCollection) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	vectorcollectionlog.Info("validate update", "name", r.Name)
	return admission.Warnings{}, ValidateVectorCollectionSpec(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *VectorCollection) ValidateDelete() (admission.Warnings, error) {
	vectorcollectionlog.Info("validate delete", "name", r.Name)
	return nil, nil
}
