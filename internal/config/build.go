package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// DefaultProperties are not overridable by the user
var DefaultProperties = map[string]string{
	"hazelcast.cluster.version.auto.upgrade.enabled": "true",
	// https://docs.hazelcast.com/hazelcast/latest/kubernetes/kubernetes-auto-discovery#configuration
	// We added the following properties to here with their default values, because DefaultProperties cannot be overridden
	"hazelcast.persistence.auto.cluster.state": "true",
}

func HazelcastConfig(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) ([]byte, error) {
	cfg := HazelcastBasicConfig(h)

	// For backward compatibility purpose; if the BaseDir is not equal to the expected, keep set it equal to MountPath
	if h.Spec.Persistence.IsEnabled() {
		existingConfig, err := getHazelcastConfig(ctx, c, types.NamespacedName{Name: h.Name, Namespace: h.Namespace})
		if err != nil {
			logger.V(util.WarnLevel).Info(fmt.Sprintf("failed to fetch old config secret %v", err))
		}
		if existingConfig != nil && existingConfig.Hazelcast.Persistence.BaseDir != n.BaseDir {
			cfg.Persistence.BaseDir = n.PersistenceMountPath
			cfg.Persistence.BackupDir = path.Join(n.PersistenceMountPath, "hot-backup")
		}
	}

	if h.Spec.Properties != nil {
		h.Spec.Properties = mergeProperties(logger, h.Spec.Properties)
	} else {
		h.Spec.Properties = make(map[string]string)
		for k, v := range DefaultProperties {
			h.Spec.Properties[k] = v
		}
	}
	// Temp solution for Tiered Storage provided by the core team.
	// To enable dynamic TS maps, the startup condition enabling TS service had to be changed.
	// Prior to this change, the service got initialized only if a TS map was present in the static configuration.
	// This forced our cloud offering to define a "fake" TS map in the static configuration.
	// Starting with this change, the TS service can be initialized with -Dhazelcast.tiered.store.force.enabled=true.
	if h.Spec.IsTieredStorageEnabled() {
		h.Spec.Properties["hazelcast.tiered.store.force.enabled"] = "true"
	}

	fillHazelcastConfigWithProperties(&cfg, h)
	fillHazelcastConfigWithExecutorServices(&cfg, h)
	fillHazelcastConfigWithSerialization(&cfg, h)

	ml, err := filterPersistedMaps(ctx, c, h)
	if err != nil {
		return nil, err
	}
	if err := fillHazelcastConfigWithMaps(ctx, c, &cfg, h, ml); err != nil {
		return nil, err
	}

	wrl, err := filterPersistedWanReplications(ctx, c, h)
	if err != nil {
		return nil, err
	}
	fillHazelcastConfigWithWanReplications(&cfg, wrl)

	ucn, err := FilterUserCodeNamespaces(ctx, c, h)
	if err != nil {
		return nil, err
	}
	fillHazelcastConfigWithUserCodeNamespaces(&cfg, h, ucn)

	dataStructures := []client.ObjectList{
		&hazelcastv1alpha1.MultiMapList{},
		&hazelcastv1alpha1.TopicList{},
		&hazelcastv1alpha1.ReplicatedMapList{},
		&hazelcastv1alpha1.QueueList{},
		&hazelcastv1alpha1.CacheList{},
	}
	for _, ds := range dataStructures {
		filteredDSList, err := filterPersistedDS(ctx, c, h, ds)
		if err != nil {
			return nil, err
		}
		if len(filteredDSList) == 0 {
			continue
		}
		switch hazelcastv1alpha1.GetKind(filteredDSList[0]) {
		case "MultiMap":
			fillHazelcastConfigWithMultiMaps(&cfg, filteredDSList)
		case "Topic":
			fillHazelcastConfigWithTopics(&cfg, filteredDSList)
		case "ReplicatedMap":
			fillHazelcastConfigWithReplicatedMaps(&cfg, filteredDSList)
		case "Queue":
			fillHazelcastConfigWithQueues(&cfg, filteredDSList)
		case "Cache":
			fillHazelcastConfigWithCaches(&cfg, filteredDSList)
		}
	}

	if h.Spec.CustomConfig.IsEnabled() {
		customCfg, overwrite, err := ReadCustomConfigYAML(ctx, c, h)
		if err != nil {
			return nil, err
		}
		if _, err = CheckAndFetchUserPermissions(customCfg); err != nil {
			return nil, err
		}
		customCfg, err = mergeConfig(customCfg, &cfg, logger, overwrite)
		if err != nil {
			return nil, err
		}
		hzWrapper := make(map[string]interface{})
		hzWrapper["hazelcast"] = customCfg
		return yaml.Marshal(hzWrapper)
	}

	return yaml.Marshal(HazelcastWrapper{Hazelcast: cfg})
}

func HazelcastBasicConfig(h *hazelcastv1alpha1.Hazelcast) Hazelcast {
	cfg := Hazelcast{
		AdvancedNetwork: AdvancedNetwork{
			Enabled: true,
			Join: Join{
				Kubernetes: Kubernetes{
					Enabled:                 ptr.To(true),
					ServiceName:             h.Name,
					ServicePort:             n.MemberServerSocketPort,
					ServicePerPodLabelName:  n.ServicePerPodLabelName,
					ServicePerPodLabelValue: n.LabelValueTrue,
				},
			},
		},
	}

	if h.Spec.DeprecatedUserCodeDeployment != nil {
		cfg.UserCodeDeployment = UserCodeDeployment{
			Enabled: h.Spec.DeprecatedUserCodeDeployment.ClientEnabled,
		}
	}

	if h.Spec.JetEngineConfiguration.IsConfigured() {
		cfg.Jet = Jet{
			Enabled:               h.Spec.JetEngineConfiguration.Enabled,
			ResourceUploadEnabled: ptr.To(h.Spec.JetEngineConfiguration.ResourceUploadEnabled),
		}

		if h.Spec.JetEngineConfiguration.Instance.IsConfigured() {
			i := h.Spec.JetEngineConfiguration.Instance
			cfg.Jet.Instance = JetInstance{
				CooperativeThreadCount:         i.CooperativeThreadCount,
				FlowControlPeriodMillis:        &i.FlowControlPeriodMillis,
				BackupCount:                    &i.BackupCount,
				ScaleUpDelayMillis:             &i.ScaleUpDelayMillis,
				LosslessRestartEnabled:         &i.LosslessRestartEnabled,
				MaxProcessorAccumulatedRecords: i.MaxProcessorAccumulatedRecords,
			}
		}

		if h.Spec.JetEngineConfiguration.EdgeDefaults.IsConfigured() {
			e := h.Spec.JetEngineConfiguration.EdgeDefaults
			cfg.Jet.EdgeDefaults = EdgeDefaults{
				QueueSize:               e.QueueSize,
				PacketSizeLimit:         e.PacketSizeLimit,
				ReceiveWindowMultiplier: e.ReceiveWindowMultiplier,
			}
		}
	}

	if h.Spec.ExposeExternally.UsesNodeName() {
		cfg.AdvancedNetwork.Join.Kubernetes.UseNodeNameAsExternalAddress = ptr.To(true)
	}

	if h.Spec.ClusterName != "" {
		cfg.ClusterName = h.Spec.ClusterName
	}

	if h.Spec.Persistence.IsEnabled() {
		cfg.Persistence = Persistence{
			Enabled:                   ptr.To(true),
			BaseDir:                   n.BaseDir,
			BackupDir:                 path.Join(n.BaseDir, "hot-backup"),
			Parallelism:               1,
			ValidationTimeoutSec:      120,
			ClusterDataRecoveryPolicy: clusterDataRecoveryPolicy(h.Spec.Persistence.ClusterDataRecoveryPolicy),
			AutoRemoveStaleData:       ptr.To(true),
		}
		if h.Spec.Persistence.DataRecoveryTimeout != 0 {
			cfg.Persistence.ValidationTimeoutSec = h.Spec.Persistence.DataRecoveryTimeout
		}
	}

	if h.Spec.HighAvailabilityMode != "" {
		switch h.Spec.HighAvailabilityMode {
		case hazelcastv1alpha1.HighAvailabilityNodeMode:
			cfg.PartitionGroup = PartitionGroup{
				Enabled:   ptr.To(true),
				GroupType: "NODE_AWARE",
			}
		case hazelcastv1alpha1.HighAvailabilityZoneMode:
			cfg.PartitionGroup = PartitionGroup{
				Enabled:   ptr.To(true),
				GroupType: "ZONE_AWARE",
			}
		}
	}

	if h.Spec.NativeMemory.IsEnabled() {
		nativeMemory := h.Spec.NativeMemory
		cfg.NativeMemory = NativeMemory{
			Enabled:                 true,
			AllocatorType:           string(nativeMemory.AllocatorType),
			MinBlockSize:            nativeMemory.MinBlockSize,
			PageSize:                nativeMemory.PageSize,
			MetadataSpacePercentage: nativeMemory.MetadataSpacePercentage,
			Size: Size{
				Value: nativeMemory.Size.ScaledValue(resource.Mega),
				Unit:  "MEGABYTES",
			},
		}
	}

	// Member Network
	cfg.AdvancedNetwork.MemberServerSocketEndpointConfig = MemberServerSocketEndpointConfig{
		Port: PortAndPortCount{
			Port:      n.MemberServerSocketPort,
			PortCount: 1,
		},
	}

	if h.Spec.AdvancedNetwork != nil && len(h.Spec.AdvancedNetwork.MemberServerSocketEndpointConfig.Interfaces) != 0 {
		cfg.AdvancedNetwork.MemberServerSocketEndpointConfig.Interfaces = Interfaces{
			Enabled:    true,
			Interfaces: h.Spec.AdvancedNetwork.MemberServerSocketEndpointConfig.Interfaces,
		}
	}

	// Client Network
	cfg.AdvancedNetwork.ClientServerSocketEndpointConfig = ClientServerSocketEndpointConfig{
		Port: PortAndPortCount{
			Port:      n.ClientServerSocketPort,
			PortCount: 1,
		},
	}

	if h.Spec.AdvancedNetwork != nil && len(h.Spec.AdvancedNetwork.ClientServerSocketEndpointConfig.Interfaces) != 0 {
		cfg.AdvancedNetwork.ClientServerSocketEndpointConfig.Interfaces = Interfaces{
			Enabled:    true,
			Interfaces: h.Spec.AdvancedNetwork.ClientServerSocketEndpointConfig.Interfaces,
		}
	}

	// Rest Network
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.Port = PortAndPortCount{
		Port:      n.RestServerSocketPort,
		PortCount: 1,
	}
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.EndpointGroups.Persistence.Enabled = ptr.To(true)
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.EndpointGroups.HealthCheck.Enabled = ptr.To(true)
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.EndpointGroups.ClusterWrite.Enabled = ptr.To(true)

	// WAN Network
	if h.Spec.AdvancedNetwork != nil && len(h.Spec.AdvancedNetwork.WAN) > 0 {
		cfg.AdvancedNetwork.WanServerSocketEndpointConfig = make(map[string]WanPort)
		for _, w := range h.Spec.AdvancedNetwork.WAN {
			cfg.AdvancedNetwork.WanServerSocketEndpointConfig[w.Name] = WanPort{
				PortAndPortCount: PortAndPortCount{
					Port:      w.Port,
					PortCount: 1,
				},
			}
		}
	} else { //Default WAN Configuration
		cfg.AdvancedNetwork.WanServerSocketEndpointConfig = make(map[string]WanPort)
		cfg.AdvancedNetwork.WanServerSocketEndpointConfig["default"] = WanPort{
			PortAndPortCount: PortAndPortCount{
				Port:      n.WanDefaultPort,
				PortCount: 1,
			},
		}
	}

	if h.Spec.ManagementCenterConfig != nil {
		cfg.ManagementCenter = ManagementCenterConfig{
			ScriptingEnabled:  h.Spec.ManagementCenterConfig.ScriptingEnabled,
			ConsoleEnabled:    h.Spec.ManagementCenterConfig.ConsoleEnabled,
			DataAccessEnabled: h.Spec.ManagementCenterConfig.DataAccessEnabled,
		}
	}

	if h.Spec.TLS != nil && h.Spec.TLS.SecretName != "" {
		var (
			jksPath  = path.Join(n.HazelcastMountPath, "hazelcast.jks")
			password = "hazelcast"
		)
		// require MTLS for member-member communication
		cfg.AdvancedNetwork.MemberServerSocketEndpointConfig.SSL = SSL{
			Enabled:          ptr.To(true),
			FactoryClassName: "com.hazelcast.nio.ssl.BasicSSLContextFactory",
			Properties:       NewSSLProperties(jksPath, password, "TLS", hazelcastv1alpha1.MutualAuthenticationRequired),
		}
		// for client-server configuration use only server TLS
		cfg.AdvancedNetwork.ClientServerSocketEndpointConfig.SSL = SSL{
			Enabled:          ptr.To(true),
			FactoryClassName: "com.hazelcast.nio.ssl.BasicSSLContextFactory",
			Properties:       NewSSLProperties(jksPath, password, "TLS", h.Spec.TLS.MutualAuthentication),
		}
	}

	if h.Spec.SQL != nil {
		cfg.SQL = SQL{
			StatementTimeout:   h.Spec.SQL.StatementTimeout,
			CatalogPersistence: h.Spec.SQL.CatalogPersistenceEnabled,
		}
	}

	if h.Spec.IsTieredStorageEnabled() {
		cfg.LocalDevice = map[string]LocalDevice{}
		for _, ld := range h.Spec.LocalDevices {
			cfg.LocalDevice[ld.Name] = createLocalDeviceConfig(ld)
		}
	}
	if h.Spec.CPSubsystem.IsEnabled() {
		cfg.CPSubsystem = CPSubsystem{
			CPMemberCount:                     *h.Spec.ClusterSize,
			PersistenceEnabled:                true,
			BaseDir:                           n.CPBaseDir,
			GroupSize:                         h.Spec.ClusterSize,
			SessionTimeToLiveSeconds:          h.Spec.CPSubsystem.SessionTTLSeconds,
			SessionHeartbeatIntervalSeconds:   h.Spec.CPSubsystem.SessionHeartbeatIntervalSeconds,
			MissingCpMemberAutoRemovalSeconds: h.Spec.CPSubsystem.MissingCpMemberAutoRemovalSeconds,
			FailOnIndeterminateOperationState: h.Spec.CPSubsystem.FailOnIndeterminateOperationState,
			DataLoadTimeoutSeconds:            h.Spec.CPSubsystem.DataLoadTimeoutSeconds,
		}
		if h.Spec.Persistence.IsEnabled() && !h.Spec.CPSubsystem.IsPVC() {
			cfg.CPSubsystem.BaseDir = n.PersistenceMountPath + n.CPDirSuffix
		}
	}

	return cfg
}

func NewSSLProperties(path, password, protocol string, auth hazelcastv1alpha1.MutualAuthentication) SSLProperties {
	const typ = "JKS"
	switch auth {
	case hazelcastv1alpha1.MutualAuthenticationRequired:
		return SSLProperties{
			Protocol:             protocol,
			MutualAuthentication: "REQUIRED",
			// server cert + key
			KeyStoreType:     typ,
			KeyStore:         path,
			KeyStorePassword: password,
			// trusted cert pool (we use the same file for convince)
			TrustStoreType:     typ,
			TrustStore:         path,
			TrustStorePassword: password,
		}
	case hazelcastv1alpha1.MutualAuthenticationOptional:
		return SSLProperties{
			Protocol:             protocol,
			MutualAuthentication: "OPTIONAL",
			KeyStoreType:         typ,
			KeyStore:             path,
			KeyStorePassword:     password,
			TrustStoreType:       typ,
			TrustStore:           path,
			TrustStorePassword:   password,
		}
	default:
		return SSLProperties{
			Protocol:         protocol,
			KeyStoreType:     typ,
			KeyStore:         path,
			KeyStorePassword: password,
		}
	}
}

func ReadCustomConfigYAML(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) (map[string]interface{}, bool, error) {
	if h.Spec.CustomConfig.ConfigMapName != "" {
		cfgCm := &corev1.ConfigMap{}
		if err := c.Get(ctx, types.NamespacedName{Name: h.Spec.CustomConfig.ConfigMapName, Namespace: h.Namespace}, cfgCm, nil); err != nil {
			return nil, false, err
		}
		customCfg := make(map[string]interface{})
		overwrite := overwriteCustomConfig(cfgCm)
		if err := yaml.Unmarshal([]byte(cfgCm.Data[n.HazelcastCustomConfigKey]), customCfg); err != nil {
			return nil, false, err
		}
		return customCfg, overwrite, nil
	} else if h.Spec.CustomConfig.SecretName != "" {
		cfgS := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Name: h.Spec.CustomConfig.SecretName, Namespace: h.Namespace}, cfgS, nil); err != nil {
			return nil, false, err
		}
		customCfg := make(map[string]interface{})
		overwrite := overwriteCustomConfig(cfgS)
		if err := yaml.Unmarshal(cfgS.Data[n.HazelcastCustomConfigKey], customCfg); err != nil {
			return nil, false, err
		}
		return customCfg, overwrite, nil
	}
	return nil, false, errors.New("unable to read custom config YAML")
}

var wrongSecurityConfigErr = errors.New("when security and client-authentication is enabled, a user with permissions 'all' with the simple authentication method must be provided")

// CheckAndFetchUserPermissions validates the security configuration for correctness.
// Returns the user that user with that contains the role assigned to "all" permissions and nil error if security and the client-authentication is enabled;
// Returns nil user and nil error if security or client-authentication is not enabled;
// Returns and error if security and the client-authentication is enabled but no user with "all" permissions is configured.
func CheckAndFetchUserPermissions(cfg map[string]interface{}) (user map[string]interface{}, err error) {
	security, hasSec := getConfigValue[map[string]interface{}]("security", cfg)
	if !hasSec {
		return
	}
	if enabled, ok := getConfigValue[bool]("enabled", security); !enabled || !ok {
		return
	}
	ca, ok := getConfigValue[map[string]interface{}]("client-authentication", security)
	if !ok || len(ca) == 0 {
		return
	}
	cp, ok := getConfigValue[map[string]interface{}]("client-permissions", security)
	if !ok || len(cp) == 0 {
		return nil, wrongSecurityConfigErr
	}
	allP, ok := getConfigValue[map[string]interface{}]("all", cp)
	if !ok || len(allP) == 0 {
		return nil, wrongSecurityConfigErr
	}

	realms, ok := getConfigValue[[]interface{}]("realms", security)
	if !ok || len(realms) == 0 {
		return nil, wrongSecurityConfigErr
	}
	clRealmName, ok := getConfigValue[string]("realm", ca)
	if !ok {
		return nil, wrongSecurityConfigErr
	}
	clientRealm := findSecurityRealm(clRealmName, realms)
	if len(clientRealm) == 0 {
		return nil, wrongSecurityConfigErr
	}
	allRole, ok := getConfigValue[string]("principal", allP)
	if !ok {
		return nil, wrongSecurityConfigErr
	}
	usr := findUserWithRole(allRole, clientRealm)
	if len(usr) == 0 {
		return nil, wrongSecurityConfigErr
	}
	return usr, nil
}

func findUserWithRole(role string, realm map[string]interface{}) map[string]interface{} {
	auth, ok := getConfigValue[map[string]interface{}]("authentication", realm)
	if !ok {
		return nil
	}
	simpRealm, ok := getConfigValue[map[string]interface{}]("simple", auth)
	if !ok {
		return nil
	}
	usrs, ok := getConfigValue[[]interface{}]("users", simpRealm)
	if !ok || len(usrs) == 0 {
		return nil
	}
	for _, user := range usrs {
		usr := user.(map[string]interface{})
		roles, ok := getConfigValue[[]interface{}]("roles", usr)
		if !ok || len(roles) == 0 {
			return nil
		}
		for _, r := range roles {
			if r == role {
				return usr
			}
		}
	}
	return nil
}

func findSecurityRealm(name string, realms []interface{}) map[string]interface{} {
	for _, realm := range realms {
		r, ok := realm.(map[string]interface{})
		if !ok {
			continue
		}
		nm, ok := getConfigValue[string]("name", r)
		if ok && nm == name {
			return r
		}
	}
	return nil
}

func getConfigValue[T any](key string, conf map[string]interface{}) (res T, ok bool) {
	val, ok := conf[key]
	if !ok {
		return
	}
	res, ok = val.(T)
	return
}

func overwriteCustomConfig(o client.Object) bool {
	v := o.GetAnnotations()[n.HazelcastCustomConfigOverwriteAnnotation]
	return v == "true"
}

func clusterDataRecoveryPolicy(policyType hazelcastv1alpha1.DataRecoveryPolicyType) string {
	switch policyType {
	case hazelcastv1alpha1.FullRecovery:
		return "FULL_RECOVERY_ONLY"
	case hazelcastv1alpha1.MostRecent:
		return "PARTIAL_RECOVERY_MOST_RECENT"
	case hazelcastv1alpha1.MostComplete:
		return "PARTIAL_RECOVERY_MOST_COMPLETE"
	}
	return "FULL_RECOVERY_ONLY"
}

func mergeProperties(logger logr.Logger, inputProps map[string]string) map[string]string {
	m := make(map[string]string)
	for k, v := range DefaultProperties {
		m[k] = v
	}
	for k, v := range inputProps {
		// if the property provided by the user is one of the default (immutable) property,
		// ignore the provided property without overriding the default one.
		if existingVal, exist := m[k]; exist {
			if v != existingVal {
				logger.V(util.DebugLevel).Info("Property ignored", "property", k, "value", v)
			}
		} else {
			m[k] = v
		}
	}
	return m
}

func mergeConfig(cstCfg map[string]interface{}, cfg *Hazelcast, logger logr.Logger, overwrite bool) (map[string]interface{}, error) {
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	crCfg := make(map[string]interface{})
	if err = yaml.Unmarshal(out, crCfg); err != nil {
		return nil, err
	}
	if overwrite {
		deepMerge(crCfg, cstCfg)
		return crCfg, nil
	} else {
		for k, v := range crCfg {
			if _, exist := cstCfg[k]; exist {
				logger.V(util.WarnLevel).Info("Custom Config section ignored", "section", k)
			}
			cstCfg[k] = v
		}
	}
	return cstCfg, nil
}

func deepMerge(dst, src map[string]any) {
	for k := range src {
		// new section, fast copy
		if _, ok := dst[k]; !ok {
			dst[k] = src[k]
			continue
		}

		// merge maps
		if src2, ok := src[k].(map[string]any); ok {
			if dst2, ok := dst[k].(map[string]any); ok {
				deepMerge(dst2, src2)
				continue
			}
		}

		// add or overwrite value
		dst[k] = src[k]
	}
}

func getHazelcastConfig(ctx context.Context, c client.Client, nn types.NamespacedName) (*HazelcastWrapper, error) {
	cm := &corev1.Secret{}
	err := c.Get(ctx, nn, cm)
	if err != nil {
		return nil, fmt.Errorf("could not find Secret for config persistence for %v", nn)
	}

	hzConfig := &HazelcastWrapper{}
	err = yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
	if err != nil {
		return nil, fmt.Errorf("persisted Secret is not formatted correctly")
	}
	return hzConfig, nil
}

func filterPersistedMaps(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) ([]hazelcastv1alpha1.Map, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	mapList := &hazelcastv1alpha1.MapList{}
	if err := c.List(ctx, mapList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}

	l := make([]hazelcastv1alpha1.Map, 0)

	for _, mp := range mapList.Items {
		switch mp.Status.State {
		case hazelcastv1alpha1.MapPersisting, hazelcastv1alpha1.MapSuccess:
			l = append(l, mp)
		case hazelcastv1alpha1.MapFailed, hazelcastv1alpha1.MapPending:
			if spec, ok := mp.Annotations[n.LastSuccessfulSpecAnnotation]; ok {
				ms := &hazelcastv1alpha1.MapSpec{}
				err := json.Unmarshal([]byte(spec), ms)
				if err != nil {
					continue
				}
				mp.Spec = *ms
				l = append(l, mp)
			}
		default:
		}
	}
	return l, nil
}

func filterPersistedDS(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, objList client.ObjectList) ([]client.Object, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	if err := c.List(ctx, objList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}
	l := make([]client.Object, 0)
	for _, obj := range objList.(hazelcastv1alpha1.CRLister).GetItems() {
		if isDSPersisted(obj) {
			l = append(l, obj)
		}
	}
	return l, nil
}

func isDSPersisted(obj client.Object) bool {
	switch obj.(hazelcastv1alpha1.DataStructure).GetStatus().State {
	case hazelcastv1alpha1.DataStructurePersisting, hazelcastv1alpha1.DataStructureSuccess:
		return true
	case hazelcastv1alpha1.DataStructureFailed, hazelcastv1alpha1.DataStructurePending:
		if spec, ok := obj.GetAnnotations()[n.LastSuccessfulSpecAnnotation]; ok {
			if err := obj.(hazelcastv1alpha1.DataStructure).SetSpec(spec); err != nil {
				return false
			}
			return true
		}
	}
	return false
}

func filterPersistedWanReplications(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) (map[string][]hazelcastv1alpha1.WanReplication, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	wrList := &hazelcastv1alpha1.WanReplicationList{}
	if err := c.List(ctx, wrList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}

	l := make(map[string][]hazelcastv1alpha1.WanReplication, 0)
	for _, wr := range wrList.Items {
		for wanKey, mapStatus := range wr.Status.WanReplicationMapsStatus {
			hzName, _ := splitWanMapKey(wanKey)
			if hzName != h.Name {
				continue
			}
			switch mapStatus.Status {
			case hazelcastv1alpha1.WanStatusPersisting, hazelcastv1alpha1.WanStatusSuccess:
				if l[wanKey] == nil {
					l[wanKey] = make([]hazelcastv1alpha1.WanReplication, 0)
				}
				l[wanKey] = append(l[wanKey], wr)
			default: // TODO, might want to do something for the other cases
			}
		}

	}
	return l, nil
}

func FilterUserCodeNamespaces(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) ([]hazelcastv1alpha1.UserCodeNamespace, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	ucnList := &hazelcastv1alpha1.UserCodeNamespaceList{}

	if err := c.List(ctx, ucnList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}

	l := make([]hazelcastv1alpha1.UserCodeNamespace, 0)

	for _, ucn := range ucnList.Items {
		switch ucn.Status.State {
		case hazelcastv1alpha1.UserCodeNamespaceSuccess:
			l = append(l, ucn)
		case hazelcastv1alpha1.UserCodeNamespaceFailure, hazelcastv1alpha1.UserCodeNamespacePending:
			if spec, ok := ucn.Annotations[n.LastSuccessfulSpecAnnotation]; ok {
				ucns := &hazelcastv1alpha1.UserCodeNamespaceSpec{}
				err := json.Unmarshal([]byte(spec), ucns)
				if err != nil {
					continue
				}
				ucn.Spec = *ucns
				l = append(l, ucn)
			}
		default:
		}
	}
	return l, nil
}

type ucnResourceType string

const (
	jarsInZip ucnResourceType = "JARS_IN_ZIP"
	jarFile   ucnResourceType = "JAR"
	classFile ucnResourceType = "CLASS"
)

func fillHazelcastConfigWithUserCodeNamespaces(cfg *Hazelcast, h *hazelcastv1alpha1.Hazelcast, ucn []hazelcastv1alpha1.UserCodeNamespace) {
	if !h.Spec.UserCodeNamespaces.IsEnabled() {
		return
	}
	cfg.UserCodeNamespaces = UserCodeNamespaces{
		Enabled:    ptr.To(true),
		Namespaces: make(map[string][]UserCodeNamespaceResource, len(ucn)),
	}
	if h.Spec.UserCodeNamespaces.ClassFilter != nil {
		cfg.UserCodeNamespaces.ClassFilter = &JavaFilterConfig{
			Blacklist: filterList(h.Spec.UserCodeNamespaces.ClassFilter.Blacklist),
			Whitelist: filterList(h.Spec.UserCodeNamespaces.ClassFilter.Whitelist),
		}
	}
	for _, u := range ucn {
		cfg.UserCodeNamespaces.Namespaces[u.Name] = []UserCodeNamespaceResource{{
			ID:           "bundle",
			ResourceType: string(jarsInZip),
			URL:          "file://" + filepath.Join(n.UCNBucketPath, u.Name+".zip"),
		}}
	}
}

func fillHazelcastConfigWithProperties(cfg *Hazelcast, h *hazelcastv1alpha1.Hazelcast) {
	cfg.Properties = h.Spec.Properties
}

func fillHazelcastConfigWithMaps(ctx context.Context, c client.Client, cfg *Hazelcast, h *hazelcastv1alpha1.Hazelcast, ml []hazelcastv1alpha1.Map) error {
	if len(ml) != 0 {
		cfg.Map = map[string]Map{}
		for _, mcfg := range ml {
			m, err := CreateMapConfig(ctx, c, h, &mcfg)
			if err != nil {
				return err
			}
			cfg.Map[mcfg.MapName()] = m
		}
	}
	return nil
}

func fillHazelcastConfigWithWanReplications(cfg *Hazelcast, wrl map[string][]hazelcastv1alpha1.WanReplication) {
	if len(wrl) != 0 {
		cfg.WanReplication = map[string]WanReplicationConfig{}
		for wanKey, wan := range wrl {
			_, mapName := splitWanMapKey(wanKey)
			wanConfig := createWanReplicationConfig(wanKey, wan)
			cfg.WanReplication[codecTypes.DefaultWanReplicationRefName(mapName)] = wanConfig
		}
	}
}

func fillHazelcastConfigWithMultiMaps(cfg *Hazelcast, mml []client.Object) {
	if len(mml) != 0 {
		cfg.MultiMap = map[string]MultiMap{}
		for _, mm := range mml {
			mm := mm.(*hazelcastv1alpha1.MultiMap)
			mmcfg := CreateMultiMapConfig(mm)
			cfg.MultiMap[mm.GetDSName()] = mmcfg
		}
	}
}

func fillHazelcastConfigWithTopics(cfg *Hazelcast, tl []client.Object) {
	if len(tl) != 0 {
		cfg.Topic = map[string]Topic{}
		for _, t := range tl {
			t := t.(*hazelcastv1alpha1.Topic)
			tcfg := CreateTopicConfig(t)
			cfg.Topic[t.GetDSName()] = tcfg
		}
	}
}

func fillHazelcastConfigWithQueues(cfg *Hazelcast, ql []client.Object) {
	if len(ql) != 0 {
		cfg.Queue = map[string]Queue{}
		for _, q := range ql {
			q := q.(*hazelcastv1alpha1.Queue)
			qcfg := CreateQueueConfig(q)
			cfg.Queue[q.GetDSName()] = qcfg
		}
	}
}

func fillHazelcastConfigWithCaches(cfg *Hazelcast, cl []client.Object) {
	if len(cl) != 0 {
		cfg.Cache = map[string]Cache{}
		for _, c := range cl {
			c := c.(*hazelcastv1alpha1.Cache)
			ccfg := CreateCacheConfig(c)
			cfg.Cache[c.GetDSName()] = ccfg
		}
	}
}

func fillHazelcastConfigWithReplicatedMaps(cfg *Hazelcast, rml []client.Object) {
	if len(rml) != 0 {
		cfg.ReplicatedMap = map[string]ReplicatedMap{}
		for _, rm := range rml {
			rm := rm.(*hazelcastv1alpha1.ReplicatedMap)
			rmcfg := CreateReplicatedMapConfig(rm)
			cfg.ReplicatedMap[rm.GetDSName()] = rmcfg
		}
	}
}

func fillHazelcastConfigWithExecutorServices(cfg *Hazelcast, h *hazelcastv1alpha1.Hazelcast) {
	if len(h.Spec.ExecutorServices) != 0 {
		cfg.ExecutorService = map[string]ExecutorService{}
		for _, escfg := range h.Spec.ExecutorServices {
			cfg.ExecutorService[escfg.Name] = createExecutorServiceConfig(&escfg)
		}
	}

	if len(h.Spec.DurableExecutorServices) != 0 {
		cfg.DurableExecutorService = map[string]DurableExecutorService{}
		for _, descfg := range h.Spec.DurableExecutorServices {
			cfg.DurableExecutorService[descfg.Name] = createDurableExecutorServiceConfig(&descfg)
		}
	}

	if len(h.Spec.ScheduledExecutorServices) != 0 {
		cfg.ScheduledExecutorService = map[string]ScheduledExecutorService{}
		for _, sescfg := range h.Spec.ScheduledExecutorServices {
			cfg.ScheduledExecutorService[sescfg.Name] = createScheduledExecutorServiceConfig(&sescfg)
		}
	}
}

func createExecutorServiceConfig(es *hazelcastv1alpha1.ExecutorServiceConfiguration) ExecutorService {
	return ExecutorService{
		PoolSize:          es.PoolSize,
		QueueCapacity:     es.QueueCapacity,
		UserCodeNamespace: es.UserCodeNamespace,
	}
}

func createDurableExecutorServiceConfig(des *hazelcastv1alpha1.DurableExecutorServiceConfiguration) DurableExecutorService {
	return DurableExecutorService{PoolSize: des.PoolSize,
		Durability:        des.Durability,
		Capacity:          des.Capacity,
		UserCodeNamespace: des.UserCodeNamespace,
	}
}

func createScheduledExecutorServiceConfig(ses *hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) ScheduledExecutorService {
	return ScheduledExecutorService{
		PoolSize:          ses.PoolSize,
		Durability:        ses.Durability,
		Capacity:          ses.Capacity,
		CapacityPolicy:    ses.CapacityPolicy,
		UserCodeNamespace: ses.UserCodeNamespace,
	}
}

func fillHazelcastConfigWithSerialization(cfg *Hazelcast, h *hazelcastv1alpha1.Hazelcast) {
	if h.Spec.Serialization == nil {
		return
	}
	s := h.Spec.Serialization
	byteOrder := "BIG_ENDIAN"
	if s.ByteOrder == hazelcastv1alpha1.LittleEndian {
		byteOrder = "LITTLE_ENDIAN"
	}
	cfg.Serialization = Serialization{
		UseNativeByteOrder:         s.ByteOrder == hazelcastv1alpha1.NativeByteOrder,
		ByteOrder:                  byteOrder,
		DataSerializableFactories:  factories(s.DataSerializableFactories),
		PortableFactories:          factories(s.PortableFactories),
		EnableCompression:          s.EnableCompression,
		EnableSharedObject:         s.EnableSharedObject,
		OverrideDefaultSerializers: s.OverrideDefaultSerializers,
		AllowUnsafe:                s.AllowUnsafe,
		Serializers:                serializers(s.Serializers),
	}
	if s.GlobalSerializer != nil {
		cfg.Serialization.GlobalSerializer = &GlobalSerializer{
			OverrideJavaSerialization: s.GlobalSerializer.OverrideJavaSerialization,
			ClassName:                 s.GlobalSerializer.ClassName,
		}
	}
	if s.JavaSerializationFilter != nil {
		cfg.Serialization.JavaSerializationFilter = &JavaFilterConfig{
			Blacklist: filterList(s.JavaSerializationFilter.Blacklist),
			Whitelist: filterList(s.JavaSerializationFilter.Whitelist),
		}
	}
	if s.CompactSerialization != nil {
		classes := make([]string, 0, len(s.CompactSerialization.Classes))
		for _, class := range s.CompactSerialization.Classes {
			classes = append(classes, fmt.Sprintf("class: %s", class))
		}
		serializers := make([]string, 0, len(s.CompactSerialization.Serializers))
		for _, serializer := range s.CompactSerialization.Serializers {
			serializers = append(serializers, fmt.Sprintf("serializer: %s", serializer))
		}
		cfg.Serialization.CompactSerialization = &CompactSerialization{
			Serializers: serializers,
			Classes:     classes,
		}
	}
}

func filterList(jsf *hazelcastv1alpha1.SerializationFilterList) *FilterList {
	if jsf == nil {
		return nil
	}
	return &FilterList{
		Classes:  jsf.Classes,
		Packages: jsf.Packages,
		Prefixes: jsf.Prefixes,
	}
}

func serializers(srs []hazelcastv1alpha1.Serializer) []Serializer {
	var res []Serializer
	for _, sr := range srs {
		res = append(res, Serializer{
			TypeClass: sr.TypeClass,
			ClassName: sr.ClassName,
		})
	}
	return res
}

func factories(factories []string) []ClassFactories {
	var classFactories []ClassFactories
	for i, f := range factories {
		classFactories = append(classFactories, ClassFactories{
			FactoryId: int32(i),
			ClassName: f,
		})
	}
	return classFactories
}

func CreateMapConfig(ctx context.Context, c client.Client, hz *hazelcastv1alpha1.Hazelcast, m *hazelcastv1alpha1.Map) (Map, error) {
	ms := m.Spec
	mc := Map{
		BackupCount:       *ms.BackupCount,
		AsyncBackupCount:  ms.AsyncBackupCount,
		TimeToLiveSeconds: ms.TimeToLiveSeconds,
		ReadBackupData:    false,
		InMemoryFormat:    string(ms.InMemoryFormat),
		Indexes:           copyMapIndexes(ms.Indexes),
		Attributes:        attributes(ms.Attributes),
		StatisticsEnabled: true,
		DataPersistence: DataPersistence{
			Enabled: ms.PersistenceEnabled,
			Fsync:   false,
		},
		Eviction: MapEviction{
			Size:           ms.Eviction.MaxSize,
			MaxSizePolicy:  string(ms.Eviction.MaxSizePolicy),
			EvictionPolicy: string(ms.Eviction.EvictionPolicy),
		},
		UserCodeNamespace: ms.UserCodeNamespace,
	}

	mc.WanReplicationReference = wanReplicationRef(codecTypes.DefaultWanReplicationRefCodec(m))

	if ms.MapStore != nil {
		msp, err := MapStoreProperties(ctx, c, ms.MapStore.PropertiesSecretName, hz.Namespace)
		if err != nil {
			return Map{}, err
		}
		mc.MapStoreConfig = MapStoreConfig{
			Enabled:           true,
			WriteCoalescing:   ms.MapStore.WriteCoealescing,
			WriteDelaySeconds: ms.MapStore.WriteDelaySeconds,
			WriteBatchSize:    ms.MapStore.WriteBatchSize,
			ClassName:         ms.MapStore.ClassName,
			Properties:        msp,
			InitialLoadMode:   string(ms.MapStore.InitialMode),
		}

	}

	if len(ms.EntryListeners) != 0 {
		mc.EntryListeners = make([]EntryListener, 0, len(ms.EntryListeners))
		for _, el := range ms.EntryListeners {
			mc.EntryListeners = append(mc.EntryListeners, EntryListener{
				ClassName:    el.ClassName,
				IncludeValue: el.GetIncludedValue(),
				Local:        el.Local,
			})
		}
	}

	if ms.NearCache != nil {
		mc.NearCache.InMemoryFormat = string(ms.NearCache.InMemoryFormat)
		mc.NearCache.InvalidateOnChange = *ms.NearCache.InvalidateOnChange
		mc.NearCache.TimeToLiveSeconds = ms.NearCache.TimeToLiveSeconds
		mc.NearCache.MaxIdleSeconds = ms.NearCache.MaxIdleSeconds
		mc.NearCache.Eviction = NearCacheEviction{
			Size:           ms.NearCache.NearCacheEviction.Size,
			MaxSizePolicy:  string(ms.NearCache.NearCacheEviction.MaxSizePolicy),
			EvictionPolicy: string(ms.NearCache.NearCacheEviction.EvictionPolicy),
		}
		mc.NearCache.CacheLocalEntries = *ms.NearCache.CacheLocalEntries
	}

	if ms.EventJournal != nil {
		mc.EventJournal.Enabled = true
		mc.EventJournal.Capacity = ms.EventJournal.Capacity
		mc.EventJournal.TimeToLiveSeconds = ms.EventJournal.TimeToLiveSeconds
	}

	if ms.TieredStore != nil {
		mc.TieredStore.Enabled = true
		mc.TieredStore.MemoryTier.Capacity = Size{
			Value: ms.TieredStore.MemoryCapacity.Value(),
			Unit:  "BYTES",
		}
		mc.TieredStore.DiskTier.Enabled = true
		mc.TieredStore.DiskTier.DeviceName = ms.TieredStore.DiskDeviceName
	}

	if ms.MerkleTree != nil {
		mc.MerkleTree.Enabled = true
		mc.MerkleTree.Depth = ms.MerkleTree.Depth
	}

	return mc, nil
}

func createWanReplicationConfig(wanKey string, wrs []hazelcastv1alpha1.WanReplication) WanReplicationConfig {
	cfg := WanReplicationConfig{
		BatchPublisher: make(map[string]BatchPublisherConfig),
	}
	if len(wrs) != 0 {
		for _, wr := range wrs {
			bpc := CreateBatchPublisherConfig(wr)
			cfg.BatchPublisher[wr.Status.WanReplicationMapsStatus[wanKey].PublisherId] = bpc
		}
	}
	return cfg
}

func CreateBatchPublisherConfig(wr hazelcastv1alpha1.WanReplication) BatchPublisherConfig {
	bpc := BatchPublisherConfig{
		ClusterName:           wr.Spec.TargetClusterName,
		TargetEndpoints:       wr.Spec.Endpoints,
		ResponseTimeoutMillis: wr.Spec.Acknowledgement.Timeout,
		AcknowledgementType:   string(wr.Spec.Acknowledgement.Type),
		QueueCapacity:         wr.Spec.Queue.Capacity,
		QueueFullBehavior:     string(wr.Spec.Queue.FullBehavior),
		BatchSize:             wr.Spec.Batch.Size,
		BatchMaxDelayMillis:   wr.Spec.Batch.MaximumDelay,
	}
	if wr.Spec.SyncConsistencyCheckStrategy != "" {
		bpc.Sync = &Sync{
			ConsistencyCheckStrategy: string(wr.Spec.SyncConsistencyCheckStrategy),
		}
	}
	return bpc
}

func CreateMultiMapConfig(mm *hazelcastv1alpha1.MultiMap) MultiMap {
	mms := mm.Spec
	return MultiMap{
		BackupCount:       *mms.BackupCount,
		AsyncBackupCount:  mms.AsyncBackupCount,
		Binary:            mms.Binary,
		CollectionType:    string(mms.CollectionType),
		StatisticsEnabled: n.DefaultMultiMapStatisticsEnabled,
		MergePolicy: MergePolicy{
			ClassName: n.DefaultMultiMapMergePolicy,
			BatchSize: n.DefaultMultiMapMergeBatchSize,
		},
		UserCodeNamespace: mm.Spec.UserCodeNamespace,
	}
}

func CreateQueueConfig(q *hazelcastv1alpha1.Queue) Queue {
	qs := q.Spec
	return Queue{
		BackupCount:             *qs.BackupCount,
		AsyncBackupCount:        qs.AsyncBackupCount,
		EmptyQueueTtl:           *qs.EmptyQueueTtlSeconds,
		MaxSize:                 qs.MaxSize,
		StatisticsEnabled:       n.DefaultQueueStatisticsEnabled,
		PriorityComparatorClass: qs.PriorityComparatorClassName,
		MergePolicy: MergePolicy{
			ClassName: n.DefaultQueueMergePolicy,
			BatchSize: n.DefaultQueueMergeBatchSize,
		},
		UserCodeNamespace: q.Spec.UserCodeNamespace,
	}
}

func CreateCacheConfig(c *hazelcastv1alpha1.Cache) Cache {
	cs := c.Spec
	cache := Cache{
		BackupCount:       *cs.BackupCount,
		AsyncBackupCount:  cs.AsyncBackupCount,
		StatisticsEnabled: n.DefaultCacheStatisticsEnabled,
		ManagementEnabled: n.DefaultCacheManagementEnabled,
		ReadThrough:       n.DefaultCacheReadThrough,
		WriteThrough:      n.DefaultCacheWriteThrough,
		InMemoryFormat:    string(cs.InMemoryFormat),
		MergePolicy: MergePolicy{
			ClassName: n.DefaultCacheMergePolicy,
			BatchSize: n.DefaultCacheMergeBatchSize,
		},
		DataPersistence: DataPersistence{
			Enabled: cs.PersistenceEnabled,
			Fsync:   false,
		},
		UserCodeNamespace: c.Spec.UserCodeNamespace,
	}
	if cs.KeyType != "" {
		cache.KeyType = ClassType{
			ClassName: cs.KeyType,
		}
	}
	if cs.ValueType != "" {
		cache.ValueType = ClassType{
			ClassName: cs.ValueType,
		}
	}
	if cs.EventJournal != nil {
		cache.EventJournal.Enabled = true
		cache.EventJournal.Capacity = cs.EventJournal.Capacity
		cache.EventJournal.TimeToLiveSeconds = cs.EventJournal.TimeToLiveSeconds
	}

	return cache
}

func CreateTopicConfig(t *hazelcastv1alpha1.Topic) Topic {
	ts := t.Spec
	return Topic{
		GlobalOrderingEnabled: ts.GlobalOrderingEnabled,
		MultiThreadingEnabled: ts.MultiThreadingEnabled,
		StatisticsEnabled:     n.DefaultTopicStatisticsEnabled,
		UserCodeNamespace:     t.Spec.UserCodeNamespace,
	}
}

func CreateReplicatedMapConfig(rm *hazelcastv1alpha1.ReplicatedMap) ReplicatedMap {
	rms := rm.Spec
	return ReplicatedMap{
		InMemoryFormat:    string(rms.InMemoryFormat),
		AsyncFillup:       *rms.AsyncFillup,
		StatisticsEnabled: n.DefaultReplicatedMapStatisticsEnabled,
		MergePolicy: MergePolicy{
			ClassName: n.DefaultReplicatedMapMergePolicy,
			BatchSize: n.DefaultReplicatedMapMergeBatchSize,
		},
		UserCodeNamespace: rm.Spec.UserCodeNamespace,
	}
}

func attributes(attributes []hazelcastv1alpha1.AttributeConfig) []Attribute {
	var att []Attribute

	for _, a := range attributes {
		att = append(att, Attribute{
			Name:               a.Name,
			ExtractorClassName: a.ExtractorClassName,
		})
	}

	return att
}

func wanReplicationRef(ref codecTypes.WanReplicationRef) map[string]WanReplicationReference {
	return map[string]WanReplicationReference{
		ref.Name: {
			MergePolicyClassName: ref.MergePolicyClassName,
			RepublishingEnabled:  ref.RepublishingEnabled,
			Filters:              ref.Filters,
		},
	}
}

func createLocalDeviceConfig(ld hazelcastv1alpha1.LocalDeviceConfig) LocalDevice {
	return LocalDevice{
		BaseDir: path.Join(n.TieredStorageBaseDir, ld.Name),
		Capacity: Size{
			Value: ld.PVC.RequestStorage.Value(),
			Unit:  "BYTES",
		},
		BlockSize:          ld.BlockSize,
		ReadIOThreadCount:  ld.ReadIOThreadCount,
		WriteIOThreadCount: ld.WriteIOThreadCount,
	}
}

func splitWanMapKey(key string) (hzName string, mapName string) {
	list := strings.Split(key, "__")
	return list[0], list[1]
}

func copyMapIndexes(idx []hazelcastv1alpha1.IndexConfig) []MapIndex {
	if idx == nil {
		return nil
	}
	ics := make([]MapIndex, len(idx))
	for i, index := range idx {
		ics[i].Type = string(index.Type)
		ics[i].Attributes = index.Attributes
		ics[i].Name = index.Name
		if index.BitmapIndexOptions != nil {
			ics[i].BitmapIndexOptions.UniqueKey = index.BitmapIndexOptions.UniqueKey
			ics[i].BitmapIndexOptions.UniqueKeyTransformation = string(index.BitmapIndexOptions.UniqueKeyTransition)
		}
	}

	return ics
}

func MapStoreProperties(ctx context.Context, c client.Client, sn, ns string) (map[string]string, error) {
	if sn == "" {
		return nil, nil
	}
	s := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: sn, Namespace: ns}, s)
	if err != nil {
		return nil, err
	}

	props := map[string]string{}
	for k, v := range s.Data {
		props[k] = string(v)
	}
	return props, nil
}
