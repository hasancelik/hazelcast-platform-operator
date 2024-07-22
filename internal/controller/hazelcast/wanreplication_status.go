package hazelcast

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
)

type WanRepStatusApplier interface {
	WanRepStatusApply(ms *hazelcastv1alpha1.WanReplicationStatus)
}

type withWanRepState hazelcastv1alpha1.WanStatus

func (w withWanRepState) WanRepStatusApply(ms *hazelcastv1alpha1.WanReplicationStatus) {
	ms.Status = hazelcastv1alpha1.WanStatus(w)
}

type withWanRepFailedState string

func (w withWanRepFailedState) WanRepStatusApply(ms *hazelcastv1alpha1.WanReplicationStatus) {
	ms.Status = hazelcastv1alpha1.WanStatusFailed
	ms.Message = string(w)
}

type withWanRepMessage string

func (w withWanRepMessage) WanRepStatusApply(ms *hazelcastv1alpha1.WanReplicationStatus) {
	ms.Message = string(w)
}

func updateWanStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanReplication, recOption controller.ReconcilerOption, options ...WanRepStatusApplier) (ctrl.Result, error) {
	for _, applier := range options {
		applier.WanRepStatusApply(&wan.Status)
	}

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if wan.Status.Status == hazelcastv1alpha1.WanStatusPending || wan.Status.Status == hazelcastv1alpha1.WanStatusPersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}

type WanMapStatusApplier interface {
	WanMapStatusApply(ms *hazelcastv1alpha1.WanReplicationMapStatus)
}

type wanMapFailedStatus string

func (w wanMapFailedStatus) WanMapStatusApply(ms *hazelcastv1alpha1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1alpha1.WanStatusFailed
	ms.Message = string(w)
}

type wanMapPersistingStatus struct {
	publisherId, resourceName string
}

func (w wanMapPersistingStatus) WanMapStatusApply(ms *hazelcastv1alpha1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1alpha1.WanStatusPersisting
	ms.PublisherId = w.publisherId
	ms.Message = ""
	ms.ResourceName = w.resourceName
}

type wanMapSuccessStatus struct{}

func (w wanMapSuccessStatus) WanMapStatusApply(ms *hazelcastv1alpha1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1alpha1.WanStatusSuccess
}

type wanMapTerminatingStatus struct{}

func (w wanMapTerminatingStatus) WanMapStatusApply(ms *hazelcastv1alpha1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1alpha1.WanStatusTerminating
}

func patchWanMapStatuses(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanReplication, options map[string]WanMapStatusApplier) error {
	if wan.Status.WanReplicationMapsStatus == nil {
		wan.Status.WanReplicationMapsStatus = make(map[string]hazelcastv1alpha1.WanReplicationMapStatus)
	}

	for mapWanKey, applier := range options {
		if _, ok := wan.Status.WanReplicationMapsStatus[mapWanKey]; !ok {
			wan.Status.WanReplicationMapsStatus[mapWanKey] = hazelcastv1alpha1.WanReplicationMapStatus{}
		}
		status := wan.Status.WanReplicationMapsStatus[mapWanKey]
		applier.WanMapStatusApply(&status)
		wan.Status.WanReplicationMapsStatus[mapWanKey] = status
	}

	wan.Status.Status = wanStatusFromMapStatuses(wan.Status.WanReplicationMapsStatus)
	if wan.Status.Status == hazelcastv1alpha1.WanStatusSuccess {
		wan.Status.Message = ""
	}

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return nil
		}
		return err
	}

	return nil
}

func wanStatusFromMapStatuses(statuses map[string]hazelcastv1alpha1.WanReplicationMapStatus) hazelcastv1alpha1.WanStatus {
	set := wanStatusSet(statuses, hazelcastv1alpha1.WanStatusSuccess)

	_, successOk := set[hazelcastv1alpha1.WanStatusSuccess]
	_, failOk := set[hazelcastv1alpha1.WanStatusFailed]
	_, persistingOk := set[hazelcastv1alpha1.WanStatusPersisting]

	if successOk && len(set) == 1 {
		return hazelcastv1alpha1.WanStatusSuccess
	}

	if persistingOk {
		return hazelcastv1alpha1.WanStatusPersisting
	}

	if failOk {
		return hazelcastv1alpha1.WanStatusFailed
	}

	return hazelcastv1alpha1.WanStatusPending

}

func wanStatusSet(statusMap map[string]hazelcastv1alpha1.WanReplicationMapStatus, checkStatuses ...hazelcastv1alpha1.WanStatus) map[hazelcastv1alpha1.WanStatus]struct{} {
	statusSet := map[hazelcastv1alpha1.WanStatus]struct{}{}

	for _, v := range statusMap {
		statusSet[v.Status] = struct{}{}
	}
	return statusSet
}

func deleteWanMapStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanReplication, mapWanKey string) error {
	if wan.Status.WanReplicationMapsStatus == nil {
		return nil
	}

	delete(wan.Status.WanReplicationMapsStatus, mapWanKey)

	if err := c.Status().Update(ctx, wan); err != nil {
		return err
	}

	return nil
}

func updateWanMapStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanReplication, mapWanKey string, applier WanMapStatusApplier) error {
	if wan.Status.WanReplicationMapsStatus == nil {
		return nil
	}

	if _, ok := wan.Status.WanReplicationMapsStatus[mapWanKey]; !ok {
		wan.Status.WanReplicationMapsStatus[mapWanKey] = hazelcastv1alpha1.WanReplicationMapStatus{}
	}
	status := wan.Status.WanReplicationMapsStatus[mapWanKey]
	applier.WanMapStatusApply(&status)
	wan.Status.WanReplicationMapsStatus[mapWanKey] = status

	if err := c.Status().Update(ctx, wan); err != nil {
		return err
	}

	return nil
}

func updateWanReplicationMemberStatus(ctx context.Context, c client.Client, sr hzclient.StatusServiceRegistry, wan *hazelcastv1alpha1.WanReplication, hzClientMap map[string][]hazelcastv1alpha1.Map) error {
	for hz, maps := range hzClientMap {
		name := types.NamespacedName{
			Namespace: wan.Namespace,
			Name:      hz,
		}

		statusService, ok := sr.Get(name)
		if !ok {
			return fmt.Errorf("get Hazelcast Status Service failed")
		}

		members := statusService.GetStatus().MemberDataMap
		for uuid := range members {
			state, err := statusService.GetTimedMemberState(ctx, uuid)
			if err != nil {
				return err
			}
			for wn, wanStats := range state.TimedMemberState.MemberState.WanStats {
				m, ok := findMapByWanName(wn, maps)
				if !ok {
					continue
				}
				mapStatusId, ok := findWanMapStatusId(hz, m, wan.Status.WanReplicationMapsStatus)
				if !ok {
					continue
				}
				mapStatus := wan.Status.WanReplicationMapsStatus[mapStatusId]
				publisherStats, ok := wanStats[mapStatus.PublisherId]
				if ok {
					if mapStatus.MembersStatus == nil {
						mapStatus.MembersStatus = make(map[string]hazelcastv1alpha1.WanMemberStatus)
					}
					mapStatus.MembersStatus[uuid.String()] = hazelcastv1alpha1.WanMemberStatus{
						IsConnected: publisherStats.IsConnected,
						State:       publisherStats.State,
					}
					wan.Status.WanReplicationMapsStatus[mapStatusId] = mapStatus
				}
			}
		}
	}

	return c.Status().Update(ctx, wan)
}

func findMapByWanName(wn string, maps []hazelcastv1alpha1.Map) (hazelcastv1alpha1.Map, bool) {
	for _, m := range maps {
		if wn == wanName(m.MapName()) {
			return m, true
		}
	}
	return hazelcastv1alpha1.Map{}, false
}

func findWanMapStatusId(hz string, m hazelcastv1alpha1.Map, statuses map[string]hazelcastv1alpha1.WanReplicationMapStatus) (string, bool) {
	for mapStatusId := range statuses {
		if mapStatusId == fmt.Sprintf("%s__%s", hz, m.MapName()) {
			return mapStatusId, true
		}
	}
	return "", false
}
