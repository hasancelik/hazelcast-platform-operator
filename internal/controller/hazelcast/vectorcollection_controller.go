package hazelcast

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

// VectorCollectionReconciler reconciles a VectorCollection object
type VectorCollectionReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
	clientRegistry   hzclient.ClientRegistry
}

func NewVectorCollectionReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cr *hzclient.HazelcastClientRegistry) *VectorCollectionReconciler {
	return &VectorCollectionReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
		clientRegistry:   cr,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=vectorcollections,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=vectorcollections/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=vectorcollections/finalizers,verbs=update

func (r *VectorCollectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-vector-collection", req.NamespacedName)
	v := &hazelcastv1alpha1.VectorCollection{}
	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, v, r.Update, r.clientRegistry, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	if err = hazelcastv1alpha1.ValidateVectorCollectionSpec(v); err != nil {
		return updateDSStatus(ctx, r.Client, v, recoptions.Error(err),
			withDSFailedState(err.Error()))
	}
	ms, err := r.ReconcileVectorCollectionConfig(ctx, v, cl, logger)
	if err != nil {
		return updateDSStatus(ctx, r.Client, v, recoptions.RetryAfter(retryAfterForDataStructures),
			withDSState(hazelcastv1alpha1.DataStructurePending),
			withDSMessage(err.Error()),
			withDSMemberStatuses(ms))
	}
	requeue, err := updateDSStatus(ctx, r.Client, v, recoptions.RetryAfter(1*time.Second),
		withDSState(hazelcastv1alpha1.DataStructurePersisting),
		withDSMessage("Persisting the applied cache config."),
		withDSMemberStatuses(ms))
	if err != nil {
		return requeue, err
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, v, logger)
}

func (r *VectorCollectionReconciler) ReconcileVectorCollectionConfig(ctx context.Context, v *hazelcastv1alpha1.VectorCollection, cl hzclient.Client, logger logr.Logger) (map[string]hazelcastv1alpha1.DataStructureConfigState, error) {
	var ic []types.VectorIndexConfig
	for _, index := range v.Spec.Indexes {
		ic = append(ic, types.VectorIndexConfig{
			Name:             index.Name,
			Metric:           vectorIndexMetric(index.Metric),
			Dimension:        index.Dimension,
			MaxDegree:        index.MaxDegree,
			EfConstruction:   index.EfConstruction,
			UseDeduplication: index.UseDeduplication,
		})
	}
	req := codec.EncodeDynamicConfigAddVectorCollectionConfigRequest(v.GetDSName(), ic)
	return sendCodecRequest(ctx, cl, v, req, logger)
}

func vectorIndexMetric(metric hazelcastv1alpha1.VectorMetric) int32 {
	switch metric {
	case hazelcastv1alpha1.Euclidean:
		return 0
	case hazelcastv1alpha1.Cosine:
		return 1
	case hazelcastv1alpha1.Dot:
		return 2
	}
	return -1
}

// SetupWithManager sets up the controller with the Manager.
func (r *VectorCollectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.VectorCollection{}).
		Complete(r)
}
