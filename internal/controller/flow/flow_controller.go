package flow

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

const retryAfter = 10 * time.Second

// FlowReconciler reconciles a Flow object
type FlowReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Log              logr.Logger
	phoneHomeTrigger chan struct{}
}

func NewFlowReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}) *FlowReconciler {
	return &FlowReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=flows,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=flows/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=flows/finalizers,verbs=update

func (r *FlowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("flow", req.NamespacedName)

	flow := &hazelcastv1alpha1.Flow{}
	err := r.Client.Get(ctx, req.NamespacedName, flow)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Flow resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	err = util.AddFinalizer(ctx, r.Client, flow, logger)
	if err != nil {
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	//Check if the Flow CR is marked to be deleted
	if flow.GetDeletionTimestamp() != nil {
		// Execute finalizer's pre-delete function to delete Flow metric
		err = r.executeFinalizer(ctx, flow)
		if err != nil {
			return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	err = r.reconcileService(ctx, flow, logger)
	if err != nil {
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	err = r.reconcileIngress(ctx, flow, logger)
	if err != nil {
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	err = r.reconcileSecret(ctx, flow, logger)
	if err != nil {
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	err = r.reconcileServiceAccount(ctx, flow, logger)
	if err != nil {
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	err = r.reconcileRole(ctx, flow, logger)
	if err != nil {
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	err = r.reconcileRoleBinding(ctx, flow, logger)
	if err != nil {
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	err = r.reconcileStatefulSet(ctx, flow, logger)
	if err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		} else {
			return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
		}
	}

	var statefulSet appsv1.StatefulSet
	if err := r.Client.Get(ctx, req.NamespacedName, &statefulSet); err != nil {
		if errors.IsNotFound(err) {
			return update(ctx, r.Client, flow, recoptions.RetryAfter(retryAfter), withFlowPhase(hazelcastv1alpha1.FlowPending))
		}
		return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
	}

	if ok, err := util.CheckIfRunning(ctx, r.Client, &statefulSet, *flow.Spec.Size); !ok {
		if err != nil {
			return update(ctx, r.Client, flow, recoptions.Error(err), withFlowFailedPhase(err.Error()))
		}
		return update(ctx, r.Client, flow, recoptions.RetryAfter(retryAfter), withFlowPhase(hazelcastv1alpha1.FlowPending))
	}
	if util.IsPhoneHomeEnabled() && !recoptions.IsSuccessfullyApplied(flow) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, flow, logger)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}
	return update(ctx, r.Client, flow, recoptions.Error(err), withFlowPhase(hazelcastv1alpha1.FlowRunning))
}

func (r *FlowReconciler) updateLastSuccessfulConfiguration(ctx context.Context, f *hazelcastv1alpha1.Flow, logger logr.Logger) error {
	opResult, err := util.Update(ctx, r.Client, f, func() error {
		recoptions.InsertLastSuccessfullyAppliedSpec(f.Spec, f)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Flow Annotation", f.Name, "result", opResult)
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *FlowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Flow{}).
		Complete(r)
}
