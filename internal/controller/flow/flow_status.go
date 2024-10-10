package flow

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	controllers "github.com/hazelcast/hazelcast-platform-operator/internal/controller"
)

type FlowStatusApplier interface {
	FlowStatusApply(fs *hazelcastv1alpha1.FlowStatus)
}

type withFlowPhase hazelcastv1alpha1.FlowPhase

func (w withFlowPhase) FlowStatusApply(fs *hazelcastv1alpha1.FlowStatus) {
	fs.Phase = hazelcastv1alpha1.FlowPhase(w)
	if hazelcastv1alpha1.FlowPhase(w) == hazelcastv1alpha1.FlowRunning {
		fs.Message = ""
	}
}

type withFlowFailedPhase string

func (w withFlowFailedPhase) FlowStatusApply(fs *hazelcastv1alpha1.FlowStatus) {
	fs.Phase = hazelcastv1alpha1.FlowFailed
	fs.Message = string(w)

}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Flow resource
func update(ctx context.Context, c client.Client, flow *hazelcastv1alpha1.Flow, recOption controllers.ReconcilerOption, options ...FlowStatusApplier) (ctrl.Result, error) {
	for _, applier := range options {
		applier.FlowStatusApply(&flow.Status)
	}
	if err := c.Status().Update(ctx, flow); err != nil {
		return ctrl.Result{}, err
	}
	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if flow.Status.Phase == hazelcastv1alpha1.FlowPending {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}
