package types

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type WanReplicationRef struct {
	Name                 string
	MergePolicyClassName string
	Filters              []string
	RepublishingEnabled  bool
}

func DefaultWanReplicationRefCodec(m *hazelcastv1alpha1.Map) WanReplicationRef {
	return WanReplicationRef{
		Name:                 DefaultWanReplicationRefName(m.MapName()),
		MergePolicyClassName: n.DefaultMergePolicyClassName,
		Filters:              []string{},
		RepublishingEnabled:  true,
	}
}

func DefaultWanReplicationRefName(name string) string {
	return name + "-default"
}
