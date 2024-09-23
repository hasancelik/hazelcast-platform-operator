package config

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type RestartConfigWrapper struct {
	Hazelcast RestartConfig `yaml:"hazelcast"`
}

type RestartConfig struct {
	ClusterName        string             `yaml:"cluster-name,omitempty"`
	Jet                Jet                `yaml:"jet,omitempty"`
	UserCodeDeployment UserCodeDeployment `yaml:"user-code-deployment,omitempty"`
	Properties         map[string]string  `yaml:"properties,omitempty"`
	AdvancedNetwork    AdvancedNetwork    `yaml:"advanced-network,omitempty"`
	CPSubsystem        CPSubsystem        `yaml:"cp-subsystem,omitempty"`
	TLS                TLS                `yaml:"tls,omitempty"`
}

type TLS struct {
	SecretName string `yaml:"sercret-name,omitempty"`
}

func ForcingRestart(h *hazelcastv1alpha1.Hazelcast) RestartConfig {
	// Apart from these changes, any change in the statefulset spec, labels, annotation can force a restart.
	cfg := RestartConfig{
		AdvancedNetwork: buildAdvancedNetwork(h),
		Properties:      h.Spec.Properties,
	}
	if h.Spec.ClusterName != "" {
		cfg.ClusterName = h.Spec.ClusterName
	}
	if h.Spec.JetEngineConfiguration.IsConfigured() {
		cfg.Jet = buildJetConfig(h)
	}
	if h.Spec.CPSubsystem.IsEnabled() {
		cfg.CPSubsystem = buildCPSubsystemConfig(h)
	}
	if h.Spec.DeprecatedUserCodeDeployment != nil {
		cfg.UserCodeDeployment = UserCodeDeployment{
			Enabled: h.Spec.DeprecatedUserCodeDeployment.ClientEnabled,
		}
	}
	if h.Spec.TLS != nil {
		cfg.TLS = TLS{
			SecretName: h.Spec.TLS.SecretName,
		}
	}
	return cfg
}
