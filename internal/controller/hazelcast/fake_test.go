package hazelcast

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	types2 "github.com/hazelcast/hazelcast-go-client/types"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	types3 "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type FakeHzClientRegistry struct {
	Clients sync.Map
}

func (cr *FakeHzClientRegistry) GetOrCreate(ctx context.Context, nn types.NamespacedName) (client.Client, error) {
	c, ok := cr.Get(nn)
	if !ok {
		return c, fmt.Errorf("fake client was not set before test")
	}
	return c, nil
}

func (cr *FakeHzClientRegistry) Set(ns types.NamespacedName, cl client.Client) {
	cr.Clients.Store(ns, cl)
}

func (cr *FakeHzClientRegistry) Get(ns types.NamespacedName) (client.Client, bool) {
	if v, ok := cr.Clients.Load(ns); ok {
		return v.(client.Client), true
	}
	return nil, false
}

func (cr *FakeHzClientRegistry) Delete(ctx context.Context, ns types.NamespacedName) error {
	if c, ok := cr.Clients.LoadAndDelete(ns); ok {
		return c.(client.Client).Shutdown(ctx) //nolint:errcheck
	}
	return nil
}

type FakeHzStatusServiceRegistry struct {
	statusServices sync.Map
}

func (ssr *FakeHzStatusServiceRegistry) Create(ns types.NamespacedName, _ client.Client, l logr.Logger, channel chan event.GenericEvent) client.StatusService {
	ss, ok := ssr.Get(ns)
	if ok {
		return ss
	}
	ssr.statusServices.Store(ns, ss)
	ss.Start()
	return ss
}

func (ssr *FakeHzStatusServiceRegistry) Set(ns types.NamespacedName, ss client.StatusService) {
	ssr.statusServices.Store(ns, ss)
}

func (ssr *FakeHzStatusServiceRegistry) Get(ns types.NamespacedName) (client.StatusService, bool) {
	if v, ok := ssr.statusServices.Load(ns); ok {
		return v.(client.StatusService), ok
	}
	return nil, false
}

func (ssr *FakeHzStatusServiceRegistry) Delete(ns types.NamespacedName) {
	if ss, ok := ssr.statusServices.LoadAndDelete(ns); ok {
		ss.(client.StatusService).Stop()
	}
}

type FakeHzStatusService struct {
	Status              *client.Status
	TimedMemberStateMap map[types2.UUID]*types3.TimedMemberStateWrapper
	TStart              func()
	TUpdateMembers      func(ss *FakeHzStatusService, ctx context.Context)
}

func (ss *FakeHzStatusService) Start() {
	if ss.TStart != nil {
		ss.TStart()
	}
}

func (ss *FakeHzStatusService) GetStatus() *client.Status {
	return ss.Status
}

func (ss *FakeHzStatusService) UpdateMembers(ctx context.Context) {
	if ss.TUpdateMembers != nil {
		ss.TUpdateMembers(ss, ctx)
	}
}

func (ss *FakeHzStatusService) GetTimedMemberState(_ context.Context, uuid types2.UUID) (*types3.TimedMemberStateWrapper, error) {
	if state, ok := ss.TimedMemberStateMap[uuid]; ok {
		return state, nil
	}
	return nil, nil
}

func (ss *FakeHzStatusService) Stop() {
}
