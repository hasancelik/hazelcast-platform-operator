package hazelcast

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	errs "errors"
	"fmt"
	"hash/crc32"
	"net"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	proto "github.com/hazelcast/hazelcast-go-client"
	clientTypes "github.com/hazelcast/hazelcast-go-client/types"
	"github.com/hazelcast/hazelcast-platform-operator/agent/init/compound"
	downloadurl "github.com/hazelcast/hazelcast-platform-operator/agent/init/file_download_url"
	downloadbucket "github.com/hazelcast/hazelcast-platform-operator/agent/init/jar_download_bucket"
	"github.com/hazelcast/hazelcast-platform-operator/agent/init/restore"
	"github.com/pavlo-v-chernykh/keystore-go/v4"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// Environment variables used for Hazelcast cluster configuration
const (
	// hzLicenseKey License key for Hazelcast cluster
	hzLicenseKey = "HZ_LICENSEKEY"
	// JavaOpts java options for Hazelcast
	JavaOpts = "JAVA_OPTS"
)

// DefaultJavaOptions are overridable by the user
var DefaultJavaOptions = map[string]string{
	"-Dhazelcast.stale.join.prevention.duration.seconds": "5",
}

func (r *HazelcastReconciler) executeFinalizer(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(h, n.Finalizer) {
		return nil
	}
	if err := r.deleteDependentCRs(ctx, h); err != nil {
		return fmt.Errorf("Could not delete all dependent CRs: %w", err)
	}
	if util.NodeDiscoveryEnabled() {
		if err := r.removeClusterRole(ctx, h, logger); err != nil {
			return fmt.Errorf("ClusterRole could not be removed: %w", err)
		}
		if err := r.removeClusterRoleBinding(ctx, h, logger); err != nil {
			return fmt.Errorf("ClusterRoleBinding could not be removed: %w", err)
		}
	}

	lk := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	r.statusServiceRegistry.Delete(lk)
	r.mtlsClientRegistry.Delete(lk.Namespace)
	if err := r.clientRegistry.Delete(ctx, lk); err != nil {
		return fmt.Errorf("Hazelcast client could not be deleted:  %w", err)
	}

	controllerutil.RemoveFinalizer(h, n.Finalizer)
	err := r.Update(ctx, h)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *HazelcastReconciler) deleteDependentCRs(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {

	dependentCRs := map[string]client.ObjectList{
		"Map":               &hazelcastv1alpha1.MapList{},
		"MultiMap":          &hazelcastv1alpha1.MultiMapList{},
		"Topic":             &hazelcastv1alpha1.TopicList{},
		"ReplicatedMap":     &hazelcastv1alpha1.ReplicatedMapList{},
		"Queue":             &hazelcastv1alpha1.QueueList{},
		"Cache":             &hazelcastv1alpha1.CacheList{},
		"JetJob":            &hazelcastv1alpha1.JetJobList{},
		"UserCodeNamespace": &hazelcastv1alpha1.UserCodeNamespaceList{},
		"VectorCollection":  &hazelcastv1alpha1.VectorCollectionList{},
	}

	for crKind, crList := range dependentCRs {
		if err := r.deleteDependentCR(ctx, h, crKind, crList); err != nil {
			return err
		}
	}
	return nil
}

func (r *HazelcastReconciler) deleteDependentCR(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, crKind string, objList client.ObjectList) error {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	if err := r.Client.List(ctx, objList, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("could not get Hazelcast dependent %v resources %w", crKind, err)
	}

	dsItems := objList.(hazelcastv1alpha1.CRLister).GetItems()
	if len(dsItems) == 0 {
		return nil
	}

	g, groupCtx := errgroup.WithContext(ctx)
	for i := 0; i < len(dsItems); i++ {
		i := i
		g.Go(func() error {
			if dsItems[i].GetDeletionTimestamp() == nil {
				return util.DeleteObject(groupCtx, r.Client, dsItems[i])
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error deleting %v resources %w", crKind, err)
	}

	if err := r.Client.List(ctx, objList, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("hazelcast dependent %v resources are not deleted yet %w", crKind, err)
	}

	dsItems = objList.(hazelcastv1alpha1.CRLister).GetItems()
	if len(dsItems) != 0 {
		return fmt.Errorf("hazelcast dependent %v resources are not deleted yet", crKind)
	}

	return nil
}

func (r *HazelcastReconciler) removeClusterRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	clusterRole := &rbacv1.ClusterRole{}
	err := r.Get(ctx, client.ObjectKey{Name: h.ClusterScopedName()}, clusterRole)
	if err != nil && kerrors.IsNotFound(err) {
		logger.V(util.DebugLevel).Info("ClusterRole is not created yet. Or it is already removed.")
		return nil
	}

	err = r.Delete(ctx, clusterRole)
	if err != nil {
		return fmt.Errorf("failed to clean up ClusterRole: %w", err)
	}
	logger.V(util.DebugLevel).Info("ClusterRole removed successfully")
	return nil
}

func (r *HazelcastReconciler) removeClusterRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	crb := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: h.ClusterScopedName()}, crb)
	if err != nil && kerrors.IsNotFound(err) {
		logger.V(util.DebugLevel).Info("ClusterRoleBinding is not created yet. Or it is already removed.")
		return nil
	}

	err = r.Delete(ctx, crb)
	if err != nil {
		return fmt.Errorf("failed to clean up ClusterRoleBinding: %w", err)
	}
	logger.V(util.DebugLevel).Info("ClusterRoleBinding removed successfully")
	return nil
}

func (r *HazelcastReconciler) reconcileClusterRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.ClusterScopedName(),
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get"},
			},
		}
		return nil
	})

	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ClusterRole", h.ClusterScopedName(), "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.Name,
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	err := controllerutil.SetControllerReference(h, role, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Role: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "pods", "services"},
				Verbs:     []string{"get", "list"},
			},
		}
		if h.Spec.Persistence.IsEnabled() {
			role.Rules = append(role.Rules, rbacv1.PolicyRule{
				APIGroups: []string{"apps"},
				Resources: []string{"statefulsets"},
				Verbs:     []string{"watch", "list"},
			})
		}
		return nil
	})

	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Role", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileServiceAccount(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	// do not create SA if user specified reference to ServiceAccountName
	if h.Spec.ServiceAccountName != "" {
		return nil
	}

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceAccountName(h),
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	err := controllerutil.SetControllerReference(h, serviceAccount, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on ServiceAccount: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, serviceAccount, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ServiceAccount", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileClusterRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	csName := h.ClusterScopedName()
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        csName,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, crb, func() error {
		crb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName(h),
				Namespace: h.Namespace,
			},
		}
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     csName,
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ClusterRoleBinding", csName, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.Name,
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	err := controllerutil.SetControllerReference(h, rb, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, rb, func() error {
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName(h),
				Namespace: h.Namespace,
			},
		}
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     h.Name,
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "RoleBinding", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileService(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	service := &corev1.Service{
		ObjectMeta: metadata(h),
		Spec: corev1.ServiceSpec{
			Selector: util.Labels(h),
		},
	}

	if serviceType(h) == corev1.ServiceTypeClusterIP {
		// We want to use headless to be compatible with Hazelcast helm chart
		service.Spec.ClusterIP = "None"
	}

	err := controllerutil.SetControllerReference(h, service, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Service: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, service, func() error {
		if h.Spec.ExposeExternally.IsEnabled() {
			switch h.Spec.ExposeExternally.DiscoveryK8ServiceType() {
			case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
				service.Labels[n.ServiceEndpointTypeLabelName] = n.ServiceEndpointTypeDiscoveryLabelValue
			default:
				delete(service.Labels, n.ServiceEndpointTypeLabelName)
			}
		} else {
			delete(service.Labels, n.ServiceEndpointTypeLabelName)
		}

		service.Spec.Type = serviceType(h)
		service.Spec.Ports = util.EnrichServiceNodePorts(discoveryServicePorts(h.Spec.AdvancedNetwork), service.Spec.Ports)

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Service", h.Name, "result", opResult)
	}

	return err
}

func (r *HazelcastReconciler) reconcileWANServices(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if h.Spec.AdvancedNetwork == nil {
		return nil
	}
	for _, w := range h.Spec.AdvancedNetwork.WAN {
		if w.ServiceType == hazelcastv1alpha1.WANServiceTypeWithExposeExternally {
			continue
		}

		service := wanService(w, h)

		err := controllerutil.SetControllerReference(h, service, r.Scheme)
		if err != nil {
			return err
		}

		opResult, err := util.CreateOrUpdate(ctx, r.Client, service, func() error {
			switch w.ServiceType {
			case hazelcastv1alpha1.WANServiceTypeLoadBalancer, hazelcastv1alpha1.WANServiceTypeNodePort:
				service.Labels[n.ServiceEndpointTypeLabelName] = n.ServiceEndpointTypeWANLabelValue
			case hazelcastv1alpha1.WANServiceTypeClusterIP:
				delete(service.Labels, n.ServiceEndpointTypeLabelName)
			}

			service.Spec.Ports = util.EnrichServiceNodePorts([]corev1.ServicePort{wanPort(w)}, service.Spec.Ports)
			if w.ServiceType == "" {
				service.Spec.Type = corev1.ServiceTypeLoadBalancer
			} else {
				service.Spec.Type = corev1.ServiceType(w.ServiceType)
			}
			return nil
		})
		if opResult != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Service", h.Name, "result", opResult)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func wanService(w hazelcastv1alpha1.WANConfig, h *hazelcastv1alpha1.Hazelcast) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.Name + "-" + w.Name,
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: util.Labels(h),
			Ports:    []corev1.ServicePort{wanPort(w)},
		},
	}
	return service
}

func wanPort(w hazelcastv1alpha1.WANConfig) corev1.ServicePort {
	return corev1.ServicePort{
		Name:        fmt.Sprintf("%s%s", n.WanPortNamePrefix, w.Name),
		Protocol:    corev1.ProtocolTCP,
		Port:        int32(w.Port),
		TargetPort:  intstr.FromInt(int(w.Port)),
		AppProtocol: ptr.To("tcp"),
	}
}

func (r *HazelcastReconciler) reconcileUnusedWANServices(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, lastSpec *hazelcastv1alpha1.HazelcastSpec) error {
	wans := findWANsToBeRemoved(h.Spec.AdvancedNetwork, lastSpec.AdvancedNetwork)
	if len(wans) == 0 {
		return nil
	}

	wanSvcLblMatcher := util.Labels(h)
	wanSvcLblMatcher[n.ServiceEndpointTypeLabelName] = n.ServiceEndpointTypeWANLabelValue

	svcList := &corev1.ServiceList{}
	if err := r.Client.List(ctx, svcList, client.MatchingLabels(wanSvcLblMatcher)); err != nil {
		return err
	}

	for _, svc := range svcList.Items {
		for _, wan := range wans {
			if fmt.Sprintf("%s-%s", h.Name, wan.Name) == svc.Name {
				if err := r.Client.Delete(ctx, &svc); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func findWANsToBeRemoved(current *hazelcastv1alpha1.AdvancedNetwork, last *hazelcastv1alpha1.AdvancedNetwork) []hazelcastv1alpha1.WANConfig {
	if current == nil && last != nil && len(last.WAN) > 0 { // remove all
		return last.WAN
	}

	if last == nil { // nothing to remove
		return []hazelcastv1alpha1.WANConfig{}
	}

	var removeList []hazelcastv1alpha1.WANConfig
	for _, c := range last.WAN {
		found := false
		for _, l := range current.WAN {
			if reflect.DeepEqual(c, l) {
				found = true
				break
			}
		}
		if !found {
			removeList = append(removeList, c)
		}
	}

	return removeList
}

func serviceType(h *hazelcastv1alpha1.Hazelcast) corev1.ServiceType {
	if h.Spec.ExposeExternally.IsEnabled() {
		return h.Spec.ExposeExternally.DiscoveryK8ServiceType()
	}
	return corev1.ServiceTypeClusterIP
}

func (r *HazelcastReconciler) reconcileServicePerPod(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !h.Spec.ExposeExternally.IsSmart() {
		// Service per pod applies only to Smart type
		return nil
	}

	for i := 0; i < int(*h.Spec.ClusterSize); i++ {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        servicePerPodName(i, h),
				Namespace:   h.Namespace,
				Labels:      servicePerPodLabels(h),
				Annotations: h.Spec.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Selector:                 servicePerPodSelector(i, h),
				PublishNotReadyAddresses: true,
			},
		}

		err := controllerutil.SetControllerReference(h, service, r.Scheme)
		if err != nil {
			return err
		}

		opResult, err := util.CreateOrUpdateForce(ctx, r.Client, service, func() error {
			switch h.Spec.ExposeExternally.MemberAccessServiceType() {
			case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
				service.Labels[n.ServiceEndpointTypeLabelName] = n.ServiceEndpointTypeMemberLabelValue
			default:
				delete(service.Labels, n.ServiceEndpointTypeLabelName)
			}

			service.Spec.Ports = util.EnrichServiceNodePorts(servicePerPodPorts(h.Spec.AdvancedNetwork), service.Spec.Ports)
			service.Spec.Type = h.Spec.ExposeExternally.MemberAccessServiceType()

			return nil
		})

		if opResult != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Service", servicePerPodName(i, h), "result", opResult)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// nodePublicAddress tries to find node public ip
func nodePublicAddress(addresses []corev1.NodeAddress) string {
	var fallbackAddress string
	// we iterate over a unordered list of addresses
	for _, address := range addresses {
		switch address.Type {
		case corev1.NodeExternalIP:
			// we found explicitly set NodeExternalIP, fast return
			return address.Address
		case corev1.NodeInternalIP:
			fallbackAddress = address.Address
		}
	}
	// no NodeExternalIP found on the list so return fallback ip
	return fallbackAddress
}

func (r *HazelcastReconciler) reconcileHazelcastEndpoints(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	// prepare a map of node addresses for fast lookup
	var nodes corev1.NodeList
	nodeAddress := make(map[string]string, len(nodes.Items))
	if util.NodeDiscoveryEnabled() {
		if err := r.Client.List(ctx, &nodes); err != nil {
			return err
		}
		for _, node := range nodes.Items {
			nodeAddress[node.Name] = nodePublicAddress(node.Status.Addresses)
		}
	}

	svcList, err := util.ListRelatedServices(ctx, r.Client, h)
	if err != nil {
		return err
	}

	reconciledHzEndpointNames := make(map[string]any)

	for _, svc := range svcList.Items {
		endpointType, ok := svc.Labels[n.ServiceEndpointTypeLabelName]
		if !ok {
			continue
		}

		var hzEndpoints []*hazelcastv1alpha1.HazelcastEndpoint

		switch endpointType {
		case n.ServiceEndpointTypeDiscoveryLabelValue:
			for _, port := range svc.Spec.Ports {
				endpointNn := types.NamespacedName{
					Name:      svc.Name,
					Namespace: svc.Namespace,
				}

				if port.Name == n.HazelcastPortName {
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeDiscovery, port.Port))
				} else if port.Name == n.WanDefaultPortName {
					endpointNn.Name = fmt.Sprintf("%s-%s", endpointNn.Name, "wan")
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeWAN, port.Port))
				} else if strings.HasPrefix(port.Name, "wan") {
					endpointNn.Name = fmt.Sprintf("%s-%s", endpointNn.Name, port.Name)
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeWAN, port.Port))
				}
			}
		case n.ServiceEndpointTypeMemberLabelValue:
			for _, port := range svc.Spec.Ports {
				endpointNn := types.NamespacedName{
					Name:      svc.Name,
					Namespace: svc.Namespace,
				}

				if port.Name == n.HazelcastPortName {
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeMember, port.Port))
				} else if port.Name == n.WanDefaultPortName {
					endpointNn.Name = fmt.Sprintf("%s-wan", endpointNn.Name)
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeWAN, port.Port))
				} else if strings.HasPrefix(port.Name, "wan") {
					endpointNn.Name = fmt.Sprintf("%s-%s", endpointNn.Name, port.Name)
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeWAN, port.Port))
				}
			}
		case n.ServiceEndpointTypeWANLabelValue:
			for _, port := range svc.Spec.Ports {
				endpointNn := types.NamespacedName{
					Name:      svc.Name,
					Namespace: svc.Namespace,
				}
				endpointNn.Name = fmt.Sprintf("%s-%s", endpointNn.Name, port.Name)
				hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeWAN, port.Port))
			}
		default:
			return fmt.Errorf("service endpoint type label values '%s' is not matched", endpointType)
		}

		for _, hzEndpoint := range hzEndpoints {
			reconciledHzEndpointNames[hzEndpoint.Name] = struct{}{}

			hzEndpointCopy := hzEndpoint.DeepCopy()
			opResult, err := util.CreateOrUpdateForce(ctx, r.Client, hzEndpoint, func() error {
				err := controllerutil.SetOwnerReference(&svc, hzEndpoint, r.Scheme)
				if err != nil {
					return err
				}

				for a, v := range hzEndpoint.ObjectMeta.Annotations {
					hzEndpoint.Annotations[a] = v
				}

				for l, v := range hzEndpoint.ObjectMeta.Labels {
					hzEndpoint.Labels[l] = v
				}

				// set the reconciled spec
				hzEndpoint.Spec = hzEndpointCopy.Spec

				return nil
			})
			if opResult != controllerutil.OperationResultNone {
				logger.Info("Operation result", "HazelcastEndpoint", hzEndpoint.Name, "result", opResult)
			}
			if err != nil {
				return err
			}

			// set external address depending on parent service type
			switch svc.Spec.Type {
			case corev1.ServiceTypeNodePort:
				// search for the first node ip of the first pod
				var pods corev1.PodList
				if err := r.Client.List(ctx, &pods, client.MatchingLabels(svc.Spec.Selector)); err != nil {
					return err
				}

				address := "*"
				if len(pods.Items) > 0 {
					address = nodeAddress[pods.Items[0].Spec.NodeName]
				}

				// NodePorts get address from svc .nodePort property
				for _, port := range svc.Spec.Ports {
					if port.Port == hzEndpoint.Spec.Port {
						hzEndpoint.Status.Address = fmt.Sprintf("%s:%d", address, port.NodePort)
						break
					}
				}

			case corev1.ServiceTypeLoadBalancer:
				// LoadBalancers get address from ingress status property
				hzEndpoint.SetAddress(util.GetExternalAddress(&svc))
			}

			err = r.Client.Status().Update(ctx, hzEndpoint)
			if err != nil {
				return err
			}
		}
	}

	// Delete the leftover HazelcastEndpoints if any.
	// The leftover resources take place after disabling the exposeExternally as an update.
	hzEndpointList := hazelcastv1alpha1.HazelcastEndpointList{}
	nsOpt := client.InNamespace(h.Namespace)
	lblOpt := client.MatchingLabels(util.Labels(h))
	if err := r.Client.List(ctx, &hzEndpointList, nsOpt, lblOpt); err != nil {
		return err
	}
	for _, hzEndpoint := range hzEndpointList.Items {
		if _, ok := reconciledHzEndpointNames[hzEndpoint.Name]; !ok {
			logger.Info("Deleting leftover HazelcastEndpoint", "name", hzEndpoint.Name)
			err := r.Client.Delete(ctx, &hzEndpoint)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *HazelcastReconciler) reconcileUnusedServicePerPod(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {
	var s int
	if h.Spec.ExposeExternally.IsSmart() {
		s = int(*h.Spec.ClusterSize)
	}

	// Delete unused services (when the cluster was scaled down)
	// The current number of service per pod is always stored in the StatefulSet annotations
	sts := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: h.Name, Namespace: h.Namespace}, sts)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Not found, StatefulSet is not created yet, no need to delete any services
			return nil
		}
		return err
	}
	p, err := strconv.Atoi(sts.ObjectMeta.Annotations[n.ServicePerPodCountAnnotation])
	if err != nil {
		// Annotation not found, no need to delete any services
		return nil
	}

	for i := s; i < p; i++ {
		s := &corev1.Service{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: servicePerPodName(i, h), Namespace: h.Namespace}, s)
		if err != nil {
			if kerrors.IsNotFound(err) {
				// Not found, no need to remove the service
				continue
			}
			return err
		}
		err = r.Client.Delete(ctx, s)
		if err != nil {
			if kerrors.IsNotFound(err) {
				// Not found, no need to remove the service
				continue
			}
			return err
		}
	}

	return nil
}

func servicePerPodName(i int, h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s-%d", h.Name, i)
}

func servicePerPodSelector(i int, h *hazelcastv1alpha1.Hazelcast) map[string]string {
	ls := util.Labels(h)
	ls[n.PodNameLabel] = servicePerPodName(i, h)
	return ls
}

func servicePerPodLabels(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	ls := labels(h)
	ls[n.ServicePerPodLabelName] = n.LabelValueTrue
	return ls
}

func servicePerPodPorts(an *hazelcastv1alpha1.AdvancedNetwork) []corev1.ServicePort {
	p := []corev1.ServicePort{
		clientPort(),
	}

	if an != nil && len(an.WAN) > 0 {
		for _, w := range an.WAN {
			if w.ServiceType == hazelcastv1alpha1.WANServiceTypeWithExposeExternally {
				p = append(p, wanPort(w))
			}
		}
	}

	return p
}

func discoveryServicePorts(an *hazelcastv1alpha1.AdvancedNetwork) []corev1.ServicePort {
	p := []corev1.ServicePort{
		clientPort(),
		{
			Name:        n.MemberPortName,
			Protocol:    corev1.ProtocolTCP,
			Port:        n.MemberServerSocketPort,
			TargetPort:  intstr.FromInt(n.MemberServerSocketPort),
			AppProtocol: ptr.To("tcp"),
		},
		{
			Name:        n.RestPortName,
			Protocol:    corev1.ProtocolTCP,
			Port:        n.RestServerSocketPort,
			TargetPort:  intstr.FromInt(n.RestServerSocketPort),
			AppProtocol: ptr.To("tcp"),
		},
	}

	if an != nil && len(an.WAN) > 0 {
		for _, w := range an.WAN {
			if w.ServiceType == hazelcastv1alpha1.WANServiceTypeWithExposeExternally {
				p = append(p, wanPort(w))
			}
		}
	} else {
		p = append(p, defaultWANPort())
	}

	return p
}

func clientPort() corev1.ServicePort {
	return corev1.ServicePort{
		Name:        n.HazelcastPortName,
		Port:        n.DefaultHzPort,
		Protocol:    corev1.ProtocolTCP,
		TargetPort:  intstr.FromString(n.Hazelcast),
		AppProtocol: ptr.To("tcp"),
	}
}

func defaultWANPort() corev1.ServicePort {
	return corev1.ServicePort{
		Name:        n.WanDefaultPortName,
		Protocol:    corev1.ProtocolTCP,
		AppProtocol: ptr.To("tcp"),
		Port:        n.WanDefaultPort,
		TargetPort:  intstr.FromInt(n.WanDefaultPort),
	}
}

func (r *HazelcastReconciler) isServicePerPodReady(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) bool {
	if !h.Spec.ExposeExternally.IsSmart() {
		// Service per pod applies only to Smart type
		return true
	}

	// Check if each service per pod is ready
	for i := 0; i < int(*h.Spec.ClusterSize); i++ {
		s := &corev1.Service{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: servicePerPodName(i, h), Namespace: h.Namespace}, s)
		if err != nil {
			// Service is not created yet
			return false
		}
		if s.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if len(s.Status.LoadBalancer.Ingress) == 0 {
				// LoadBalancer service waiting for External IP to get assigned
				return false
			}
			for _, ingress := range s.Status.LoadBalancer.Ingress {
				// Hostname is set for load-balancer ingress points that are DNS based
				// (typically AWS load-balancers)
				if ingress.Hostname != "" {
					if _, err := net.DefaultResolver.LookupHost(ctx, ingress.Hostname); err != nil {
						// Hostname does not resolve yet
						return false
					}
				}
			}
		}
	}

	return true
}

func hazelcastEndpointFromService(nn types.NamespacedName, hz *hazelcastv1alpha1.Hazelcast, endpointType hazelcastv1alpha1.HazelcastEndpointType, port int32) *hazelcastv1alpha1.HazelcastEndpoint {
	return &hazelcastv1alpha1.HazelcastEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nn.Name,
			Namespace:   nn.Namespace,
			Labels:      labels(hz),
			Annotations: hz.Spec.Annotations,
		},
		Spec: hazelcastv1alpha1.HazelcastEndpointSpec{
			Type:                  endpointType,
			Port:                  port,
			HazelcastResourceName: hz.Name,
		},
	}
}

func (r *HazelcastReconciler) reconcileInitContainerConfig(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metadata(h),
		Data:       make(map[string]string),
	}
	cm.Name = cm.Name + n.InitSuffix
	err := controllerutil.SetControllerReference(h, cm, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on ConfigMap: %w", err)
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
			cfg, err := initContainerConfig(ctx, r.Client, h, logger)
			if err != nil {
				return err
			}
			cm.Data[n.InitConfigFile] = string(cfg)
			return nil
		})
		if result != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Secret", h.Name, "result", result)
		}
		return err
	})
}

func (r *HazelcastReconciler) reconcileSecret(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	scrt := &corev1.Secret{
		ObjectMeta: metadata(h),
		Data:       make(map[string][]byte),
	}

	err := controllerutil.SetControllerReference(h, scrt, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Secret: %w", err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, err := controllerutil.CreateOrUpdate(ctx, r.Client, scrt, func() error {
			cfg, err := config.HazelcastConfig(ctx, r.Client, h, logger)
			if err != nil {
				return err
			}
			scrt.Data["hazelcast.yaml"] = cfg

			if _, ok := scrt.Data["hazelcast.jks"]; !ok {
				keystore, err := hazelcastKeystore(ctx, r.Client, h)
				if err != nil {
					return err
				}
				scrt.Data["hazelcast.jks"] = keystore
			}

			return nil
		})
		if result != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Secret", h.Name, "result", result)
		}
		return err
	})
}

func (r *HazelcastReconciler) reconcileMTLSSecret(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {
	_, err := r.mtlsClientRegistry.Create(ctx, r.Client, h.Namespace)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: h.Namespace}
	err = r.Client.Get(ctx, secretName, secret)
	if err != nil {
		return err
	}
	err = controllerutil.SetControllerReference(h, secret, r.Scheme)
	if err != nil {
		return err
	}
	_, err = util.CreateOrUpdateForce(ctx, r.Client, secret, func() error {
		return nil
	})
	return err
}

func initContainerConfig(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) ([]byte, error) {
	cfgW := compound.ConfigWrapper{
		InitContainer: &compound.Config{},
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		ns, err := config.FilterUserCodeNamespaces(ctx, c, h)
		if err != nil {
			return nil, err
		}

		if len(ns) != 0 {
			buckets := make([]downloadbucket.Cmd, 0)
			for _, ucn := range ns {
				buckets = append(buckets, downloadbucket.Cmd{
					BucketURI:   ucn.Spec.BucketConfiguration.BucketURI,
					SecretName:  ucn.Spec.BucketConfiguration.SecretName,
					Destination: filepath.Join(n.UCNBucketPath, ucn.Name+".zip"),
				})
			}
			cfgW.InitContainer = &compound.Config{
				Download: &compound.Download{
					Bundle: &compound.Bundle{
						Buckets: buckets,
					},
				},
			}
		}
	}

	ucn := h.Spec.DeprecatedUserCodeDeployment

	if h.Spec.DeprecatedUserCodeDeployment.IsBucketEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			Buckets: []downloadbucket.Cmd{
				{
					Destination: n.UserCodeBucketPath,
					SecretName:  ucn.BucketConfiguration.GetSecretName(),
					BucketURI:   ucn.BucketConfiguration.BucketURI,
				},
			},
		}
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsRemoteURLsEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			URLs: []downloadurl.Cmd{
				{
					Destination: n.UserCodeBucketPath,
					URLs:        strings.Join(ucn.RemoteURLs, ","),
				},
			},
		}
	}

	jet := h.Spec.JetEngineConfiguration

	if h.Spec.JetEngineConfiguration.IsBucketEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			Buckets: []downloadbucket.Cmd{
				{
					Destination: n.JetJobJarsPath,
					SecretName:  jet.BucketConfiguration.GetSecretName(),
					BucketURI:   jet.BucketConfiguration.BucketURI,
				},
			},
		}
	}

	if h.Spec.JetEngineConfiguration.IsRemoteURLsEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			URLs: []downloadurl.Cmd{
				{
					Destination: n.JetJobJarsPath,
					URLs:        strings.Join(jet.RemoteURLs, ","),
				},
			},
		}
	}

	if !h.Spec.Persistence.IsRestoreEnabled() {
		return yaml.Marshal(cfgW)
	}

	hzConf, err := getHazelcastConfig(ctx, c, types.NamespacedName{Name: h.Name, Namespace: h.Namespace})
	if err != nil {
		return nil, fmt.Errorf("unable to fetch existing HZ config: %v", err)
	}

	var r compound.Restore
	if h.Spec.Persistence.RestoreFromHotBackupResourceName() {
		r, err = restoreCmdForHBResource(ctx, c, h,
			types.NamespacedName{Namespace: h.Namespace, Name: h.Spec.Persistence.Restore.HotBackupResourceName}, hzConf.Hazelcast.Persistence.BaseDir)
		if err != nil {
			return nil, err
		}
	} else if h.Spec.Persistence.RestoreFromLocalBackup() {
		r = restoreLocalInitContainer(h, *h.Spec.Persistence.Restore.LocalConfiguration, hzConf.Hazelcast.Persistence.BaseDir)
	} else {
		r = restoreInitContainer(h, h.Spec.Persistence.Restore.BucketConfiguration.GetSecretName(),
			h.Spec.Persistence.Restore.BucketConfiguration.BucketURI, hzConf.Hazelcast.Persistence.BaseDir)
	}
	cfgW.InitContainer.Restore = &r

	return yaml.Marshal(cfgW)
}

func restoreCmdForHBResource(ctx context.Context, cl client.Client, h *hazelcastv1alpha1.Hazelcast, key types.NamespacedName, baseDir string) (compound.Restore, error) {
	hb := &hazelcastv1alpha1.HotBackup{}
	err := cl.Get(ctx, key, hb)
	if err != nil {
		return compound.Restore{}, err
	}

	if hb.Status.State != hazelcastv1alpha1.HotBackupSuccess {
		return compound.Restore{}, fmt.Errorf("restore hotbackup '%s' status is not %s", hb.Name, hazelcastv1alpha1.HotBackupSuccess)
	}

	var r compound.Restore
	if hb.Spec.IsExternal() {
		bucketURI := hb.Status.GetBucketURI()
		r = restoreInitContainer(h, hb.Spec.GetSecretName(), bucketURI, baseDir)
	} else {
		backupFolder := hb.Status.GetBackupFolder()
		r = restoreLocalInitContainer(h, hazelcastv1alpha1.RestoreFromLocalConfiguration{
			BackupFolder: backupFolder,
		}, baseDir)
	}

	return r, nil
}

func restoreInitContainer(h *hazelcastv1alpha1.Hazelcast, secretName, bucket, baseDir string) compound.Restore {
	return compound.Restore{
		Bucket: &restore.BucketToPVCCmd{
			Bucket:      bucket,
			Destination: baseDir,
			// Hostname:    "",
			SecretName: secretName,
			RestoreID:  h.Spec.Persistence.Restore.Hash(),
		},
	}
}

func restoreLocalInitContainer(h *hazelcastv1alpha1.Hazelcast, conf hazelcastv1alpha1.RestoreFromLocalConfiguration, destBaseDir string) compound.Restore {
	// it maybe configured by restore.localConfig.
	// HotBackup CR does not configure it. For HotBackup CR source and destination base directories are always the same.
	srcBaseDir := destBaseDir
	if conf.BaseDir != "" {
		srcBaseDir = conf.BaseDir
	}

	backupDir := n.BackupDir
	if conf.BackupDir != "" {
		backupDir = conf.BackupDir
	}

	backupFolder := conf.BackupFolder

	return compound.Restore{
		PVC: &restore.LocalInPVCCmd{
			BackupSequenceFolderName: backupFolder,
			BackupSourceBaseDir:      srcBaseDir,
			BackupDestinationBaseDir: destBaseDir,
			BackupDir:                backupDir,
			// Hostname:                 "",
			RestoreID: h.Spec.Persistence.Restore.Hash(),
		},
	}
}

func hazelcastKeystore(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) ([]byte, error) {
	var (
		store    = keystore.New()
		password = []byte("hazelcast")
	)
	if h.Spec.TLS != nil && h.Spec.TLS.SecretName != "" {
		cert, key, err := loadTLSKeyPair(ctx, c, h)
		if err != nil {
			return nil, err
		}
		err = store.SetPrivateKeyEntry("hazelcast", keystore.PrivateKeyEntry{
			CreationTime: time.Now(),
			PrivateKey:   key,
			CertificateChain: []keystore.Certificate{{
				Type:    "X509",
				Content: cert,
			}},
		}, password)
		if err != nil {
			return nil, err
		}
	}
	var b bytes.Buffer
	if err := store.Store(&b, password); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func loadTLSKeyPair(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) (cert []byte, key []byte, err error) {
	var s corev1.Secret
	err = c.Get(ctx, types.NamespacedName{Name: h.Spec.TLS.SecretName, Namespace: h.Namespace}, &s)
	if err != nil {
		return
	}
	cert, err = decodePEM(s.Data["tls.crt"], "CERTIFICATE")
	if err != nil {
		return
	}
	key, err = decodePEM(s.Data["tls.key"], "PRIVATE KEY")
	if err != nil {
		return
	}
	return
}

func decodePEM(data []byte, typ string) ([]byte, error) {
	b, _ := pem.Decode(data)
	if b == nil {
		return nil, fmt.Errorf("expected at least one pem block")
	}
	if b.Type != typ {
		return nil, fmt.Errorf("expected type %v, got %v", typ, b.Type)
	}
	return b.Bytes, nil
}

func (r *HazelcastReconciler) reconcileStatefulset(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	var terminationGracePeriodSeconds int64 = 600
	sts := &appsv1.StatefulSet{
		ObjectMeta: metadata(h),
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: util.Labels(h),
			},
			ServiceName:         h.Name,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels(h),
					Annotations: h.Spec.Annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName(h),
					SecurityContext:    podSecurityContext(),
					Containers: []corev1.Container{{
						Name: n.Hazelcast,
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/hazelcast/health/node-state",
									Port:   intstr.FromInt(n.RestServerSocketPort),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 0,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/hazelcast/health/node-state",
									Port:   intstr.FromInt(n.RestServerSocketPort),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 0,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						SecurityContext: containerSecurityContext(),
					}},
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	pvcName := n.PVCName
	if h.Spec.Persistence.RestoreFromLocalBackup() {
		pvcName = string(h.Spec.Persistence.Restore.LocalConfiguration.PVCNamePrefix)
	}

	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, sidecarContainer(h))
	sts.Spec.VolumeClaimTemplates = persistentVolumeClaims(h, pvcName)
	if h.Spec.IsTieredStorageEnabled() {
		sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, localDevicePersistentVolumeClaim(h)...)
	}

	err := controllerutil.SetControllerReference(h, sts, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Statefulset: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, sts, func() error {
		if len(sts.Spec.VolumeClaimTemplates) > 0 {
			pvcName = sts.Spec.VolumeClaimTemplates[0].Name
		}
		sts.Spec.Replicas = h.Spec.ClusterSize
		sts.ObjectMeta.Annotations = statefulSetAnnotations(sts, h)
		sts.Spec.Template.Annotations, err = podAnnotations(sts.Spec.Template.Annotations, h)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec.ServiceAccountName = serviceAccountName(h)
		sts.Spec.Template.Spec.ImagePullSecrets = h.Spec.ImagePullSecrets
		sts.Spec.Template.Spec.Containers[0].Image = h.DockerImage()
		sts.Spec.Template.Spec.Containers[0].Env = env(h)
		sts.Spec.Template.Spec.Containers[0].ImagePullPolicy = h.Spec.ImagePullPolicy
		if h.Spec.Resources != nil {
			sts.Spec.Template.Spec.Containers[0].Resources = *h.Spec.Resources
		}
		sts.Spec.Template.Spec.Containers[0].Ports = hazelcastContainerPorts(h)

		if h.Spec.Scheduling != nil {
			sts.Spec.Template.Spec.Affinity = h.Spec.Scheduling.Affinity
			sts.Spec.Template.Spec.Tolerations = h.Spec.Scheduling.Tolerations
			sts.Spec.Template.Spec.NodeSelector = h.Spec.Scheduling.NodeSelector
		}
		sts.Spec.Template.Spec.TopologySpreadConstraints = appendHAModeTopologySpreadConstraints(h)

		if semver.Compare(fmt.Sprintf("v%s", h.Spec.Version), "v5.2.0") == 1 {
			sts.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.HTTPGet.Path = "/hazelcast/health/ready"
		}

		ic, err := initContainer(h, pvcName)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec.InitContainers = []corev1.Container{ic}

		sts.Spec.Template.Spec.Volumes = volumes(h)
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = hzContainerVolumeMounts(h, pvcName)
		sts.Spec.Template.Spec.Containers[1].VolumeMounts = sidecarVolumeMounts(h, pvcName)
		if h.Spec.Agent.Resources != nil {
			sts.Spec.Template.Spec.Containers[1].Resources = *h.Spec.Agent.Resources
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", h.Name, "result", opResult)
	}
	return err
}

func persistentVolumeClaims(h *hazelcastv1alpha1.Hazelcast, pvcName string) []corev1.PersistentVolumeClaim {
	var pvcs []corev1.PersistentVolumeClaim
	if h.Spec.Persistence.IsEnabled() {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        pvcName,
				Namespace:   h.Namespace,
				Labels:      labels(h),
				Annotations: h.Spec.Annotations,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: h.Spec.Persistence.PVC.AccessModes,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *h.Spec.Persistence.PVC.RequestStorage,
					},
				},
				StorageClassName: h.Spec.Persistence.PVC.StorageClassName,
			},
		})
	}
	if h.Spec.CPSubsystem.IsEnabled() && h.Spec.CPSubsystem.IsPVC() {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        n.CPPersistenceVolumeName,
				Namespace:   h.Namespace,
				Labels:      labels(h),
				Annotations: h.Spec.Annotations,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: h.Spec.CPSubsystem.PVC.AccessModes,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *h.Spec.CPSubsystem.PVC.RequestStorage,
					},
				},
				StorageClassName: h.Spec.CPSubsystem.PVC.StorageClassName,
			},
		})
	}
	return pvcs
}

func localDevicePersistentVolumeClaim(h *hazelcastv1alpha1.Hazelcast) []corev1.PersistentVolumeClaim {
	var pvcs []corev1.PersistentVolumeClaim
	for _, localDeviceConfig := range h.Spec.LocalDevices {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      localDeviceConfig.Name,
				Namespace: h.Namespace,
				Labels:    labels(h),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: localDeviceConfig.PVC.AccessModes,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *localDeviceConfig.PVC.RequestStorage,
					},
				},
				StorageClassName: localDeviceConfig.PVC.StorageClassName,
			},
		})
	}
	return pvcs
}

func sidecarContainer(h *hazelcastv1alpha1.Hazelcast) corev1.Container {
	return corev1.Container{
		Name:  n.SidecarAgent,
		Image: h.AgentDockerImage(),
		Ports: []corev1.ContainerPort{{
			ContainerPort: n.DefaultAgentPort,
			Name:          n.SidecarAgent,
			Protocol:      corev1.ProtocolTCP,
		}},
		Args: []string{"sidecar"},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/health",
					Port:   intstr.FromInt(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 0,
			TimeoutSeconds:      10,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/health",
					Port:   intstr.FromInt(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 0,
			TimeoutSeconds:      10,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    10,
		},
		Env: []corev1.EnvVar{
			{
				Name:  "BACKUP_CA",
				Value: path.Join(n.MTLSCertPath, "ca.crt"),
			},
			{
				Name:  "BACKUP_CERT",
				Value: path.Join(n.MTLSCertPath, "tls.crt"),
			},
			{
				Name:  "BACKUP_KEY",
				Value: path.Join(n.MTLSCertPath, "tls.key"),
			},
		},
		SecurityContext: containerSecurityContext(),
	}
}

func hazelcastContainerWanPorts(h *hazelcastv1alpha1.Hazelcast) []corev1.ContainerPort {
	// If WAN is not configured, use the default port for it
	if h.Spec.AdvancedNetwork == nil || len(h.Spec.AdvancedNetwork.WAN) == 0 {
		return []corev1.ContainerPort{{
			ContainerPort: n.WanDefaultPort,
			Name:          n.WanDefaultPortName,
			Protocol:      corev1.ProtocolTCP,
		}}
	}

	var c []corev1.ContainerPort
	for _, w := range h.Spec.AdvancedNetwork.WAN {
		c = append(c, corev1.ContainerPort{
			ContainerPort: int32(int(w.Port)),
			Name:          fmt.Sprintf("%s%s", n.WanPortNamePrefix, w.Name),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	return c
}

func podSecurityContext() *corev1.PodSecurityContext {
	// Openshift assigns user and fsgroup ids itself
	if platform.GetType() == platform.OpenShift {
		return &corev1.PodSecurityContext{
			RunAsNonRoot: ptr.To(true),
		}
	}

	var u int64 = 65534
	return &corev1.PodSecurityContext{
		FSGroup:      &u,
		RunAsNonRoot: ptr.To(true),
		// Have to give userID otherwise Kubelet fails to create the pod
		// saying userID must be numberic, Hazelcast image's default userID is "hazelcast"
		// UBI images prohibits all numeric userIDs https://access.redhat.com/solutions/3103631
		RunAsUser: &u,
	}
}

func containerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		Privileged:               ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func initContainer(h *hazelcastv1alpha1.Hazelcast, pvcName string) (corev1.Container, error) {
	c := corev1.Container{
		Name:  n.InitContainer,
		Image: h.AgentDockerImage(),
		Args:  []string{"execute-multiple-commands"},
		Env: []corev1.EnvVar{
			{
				Name:  "CONFIG_FILE",
				Value: n.InitConfigDir + n.InitConfigFile,
			},
			{
				Name: "HOSTNAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		SecurityContext:          containerSecurityContext(),
		ImagePullPolicy:          corev1.PullIfNotPresent,
		VolumeMounts: []corev1.VolumeMount{

			{
				Name:      n.InitConfigMap,
				MountPath: n.InitConfigDir,
			},
		},
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsBucketEnabled() ||
		h.Spec.DeprecatedUserCodeDeployment.IsRemoteURLsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, ucdBucketAgentVolumeMount())
	}

	if h.Spec.JetEngineConfiguration.IsBucketEnabled() ||
		h.Spec.JetEngineConfiguration.IsRemoteURLsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, jetJobJarsVolumeMount())
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, ucnBucketVolumeMount())
	}

	if h.Spec.Persistence.IsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, persistenceVolumeMount(pvcName))
	}

	return c, nil
}

func persistenceVolumeMount(pvcName string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      pvcName,
		MountPath: n.PersistenceMountPath,
	}
}

func ucnBucketVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.UCNVolumeName,
		MountPath: n.UCNBucketPath,
	}
}

func ucdBucketAgentVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.UserCodeBucketVolumeName,
		MountPath: n.UserCodeBucketPath,
	}
}

func jetJobJarsVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.JetJobJarsVolumeName,
		MountPath: n.JetJobJarsPath,
	}
}

func tmpDirVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.TmpDirVolName,
		MountPath: "/tmp",
	}
}

func ucdURLAgentVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.UserCodeURLVolumeName,
		MountPath: n.UserCodeURLPath,
	}
}

func volumes(h *hazelcastv1alpha1.Hazelcast) []corev1.Volume {
	var defaultMode int32 = 420
	vols := []corev1.Volume{
		{
			Name: n.HazelcastStorageName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  h.Name,
					DefaultMode: &defaultMode,
				},
			},
		},
		{
			Name: n.InitConfigMap,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: h.Name + n.InitSuffix,
					},
					DefaultMode: &defaultMode,
				},
			},
		},
		emptyDirVolume(n.UserCodeBucketVolumeName),
		emptyDirVolume(n.UserCodeURLVolumeName),
		emptyDirVolume(n.JetJobJarsVolumeName),
		emptyDirVolume(n.TmpDirVolName),
		tlsVolume(h),
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		vols = append(vols, emptyDirVolume(n.UCNVolumeName))
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsConfigMapEnabled() {
		vols = append(vols, configMapVolumes(ucdConfigMapName(h), h.Spec.DeprecatedUserCodeDeployment.RemoteFileConfiguration)...)
	}

	if h.Spec.JetEngineConfiguration.IsConfigMapEnabled() {
		vols = append(vols, configMapVolumes(jetConfigMapName, h.Spec.JetEngineConfiguration.RemoteFileConfiguration)...)
	}

	return vols
}

func emptyDirVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func tlsVolume(_ *hazelcastv1alpha1.Hazelcast) corev1.Volume {
	return corev1.Volume{
		Name: n.MTLSCertSecretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  n.MTLSCertSecretName,
				DefaultMode: &[]int32{420}[0],
			},
		},
	}
}

type ConfigMapVolumeName func(cm string) string

func jetConfigMapName(cm string) string {
	return n.JetConfigMapNamePrefix + cm
}

func ucdConfigMapName(h *hazelcastv1alpha1.Hazelcast) ConfigMapVolumeName {
	return func(cm string) string {
		return n.UserCodeConfigMapNamePrefix + cm + h.Spec.DeprecatedUserCodeDeployment.TriggerSequence
	}
}

func configMapVolumes(nameFn ConfigMapVolumeName, rfc hazelcastv1alpha1.RemoteFileConfiguration) []corev1.Volume {
	var vols []corev1.Volume
	for _, cm := range rfc.ConfigMaps {
		vols = append(vols, corev1.Volume{
			Name: nameFn(cm),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm,
					},
					DefaultMode: &[]int32{420}[0],
				},
			},
		})
	}
	return vols
}

func sidecarVolumeMounts(h *hazelcastv1alpha1.Hazelcast, pvcName string) []corev1.VolumeMount {
	vm := []corev1.VolumeMount{
		{
			Name:      n.MTLSCertSecretName,
			MountPath: n.MTLSCertPath,
		},
		jetJobJarsVolumeMount(),
		ucdBucketAgentVolumeMount(),
	}
	if h.Spec.UserCodeNamespaces.IsEnabled() {
		vm = append(vm, ucnBucketVolumeMount())
	}
	if h.Spec.Persistence.IsEnabled() {
		vm = append(vm, persistenceVolumeMount(pvcName))
	}
	return vm
}

func hzContainerVolumeMounts(h *hazelcastv1alpha1.Hazelcast, pvcName string) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      n.HazelcastStorageName,
			MountPath: n.HazelcastMountPath,
		},
		ucdBucketAgentVolumeMount(),
		ucdURLAgentVolumeMount(),
		jetJobJarsVolumeMount(),
		// /tmp dir is overriden with emptyDir because Hazelcast fails to start with
		// read-only rootFileSystem when persistence is enabled because it tries to write
		// into /tmp dir.
		// /tmp dir is also needed for Jet Job submission and UCD from client/CLC.
		tmpDirVolumeMount(),
	}

	if h.Spec.Persistence.IsEnabled() {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      pvcName,
			MountPath: n.PersistenceMountPath,
		})
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		mounts = append(mounts, ucnBucketVolumeMount())
	}

	if h.Spec.CPSubsystem.IsEnabled() && h.Spec.CPSubsystem.IsPVC() {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      n.CPPersistenceVolumeName,
			MountPath: n.CPBaseDir,
		})
	}

	if h.Spec.IsTieredStorageEnabled() {
		mounts = append(mounts, localDeviceVolumeMounts(h)...)
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsConfigMapEnabled() {
		mounts = append(mounts,
			configMapVolumeMounts(ucdConfigMapName(h), h.Spec.DeprecatedUserCodeDeployment.RemoteFileConfiguration, n.UserCodeConfigMapPath)...)
	}

	if h.Spec.JetEngineConfiguration.IsConfigMapEnabled() {
		mounts = append(mounts,
			configMapVolumeMounts(jetConfigMapName, h.Spec.JetEngineConfiguration.RemoteFileConfiguration, n.JetJobJarsPath)...)
	}
	return mounts
}

func localDeviceVolumeMounts(h *hazelcastv1alpha1.Hazelcast) []corev1.VolumeMount {
	var vms []corev1.VolumeMount
	for _, ld := range h.Spec.LocalDevices {
		vms = append(vms, corev1.VolumeMount{
			Name:      ld.Name,
			MountPath: path.Join(n.TieredStorageBaseDir, ld.Name),
		})
	}
	return vms
}

func configMapVolumeMounts(nameFn ConfigMapVolumeName, rfc hazelcastv1alpha1.RemoteFileConfiguration, mountPath string) []corev1.VolumeMount {
	var vms []corev1.VolumeMount
	for _, cm := range rfc.ConfigMaps {
		vms = append(vms, corev1.VolumeMount{
			Name:      nameFn(cm),
			MountPath: path.Join(mountPath, cm),
		})
	}
	return vms
}

// persistenceStartupAction performs the action specified in the h.Spec.Persistence.DeprecatedStartupAction if
// the persistence is enabled and if the Hazelcast is not yet running
func (r *HazelcastReconciler) persistenceStartupAction(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !h.Spec.Persistence.IsEnabled() ||
		h.Spec.Persistence.DeprecatedStartupAction == "" ||
		h.Status.Phase == hazelcastv1alpha1.Running {
		return nil
	}
	logger.Info("Persistence enabled with startup action.", "action", h.Spec.Persistence.DeprecatedStartupAction)
	if h.Spec.Persistence.DeprecatedStartupAction == hazelcastv1alpha1.ForceStart {
		return NewRestClient(h).ForceStart(ctx)
	}
	if h.Spec.Persistence.DeprecatedStartupAction == hazelcastv1alpha1.PartialStart {
		return NewRestClient(h).PartialStart(ctx)
	}
	return nil
}

func (r *HazelcastReconciler) ensureClusterActive(ctx context.Context, client hzclient.Client, h *hazelcastv1alpha1.Hazelcast) error {
	// make sure restore is active
	if !h.Spec.Persistence.IsRestoreEnabled() {
		return nil
	}

	// make sure restore was successful
	if h.Status.Restore == (hazelcastv1alpha1.RestoreStatus{}) {
		return nil
	}

	if h.Status.Restore.State != hazelcastv1alpha1.RestoreSucceeded {
		return nil
	}

	if h.Status.Phase == hazelcastv1alpha1.Pending {
		return nil
	}

	// check if all cluster members are in passive state
	for _, member := range h.Status.Members {
		if member.State != hazelcastv1alpha1.NodeStatePassive {
			return nil
		}
	}

	svc := hzclient.NewClusterStateService(client)
	state, err := svc.ClusterState(ctx)
	if err != nil {
		return err
	}
	if state == codecTypes.ClusterStateActive {
		return nil
	}
	return svc.ChangeClusterState(ctx, codecTypes.ClusterStateActive)
}

var illegalClusterType = errs.New("only enterprise clusters are supported")

func (r *HazelcastReconciler) isEnterpriseCluster(ctx context.Context, client hzclient.Client, h *hazelcastv1alpha1.Hazelcast) (bool, error) {
	req := codec.EncodeClientAuthenticationRequest(
		h.Name,
		"",
		"",
		clientTypes.NewUUID(),
		"GOO",
		1,
		"1.5.0",
		"operator",
		[]string{})
	resp, err := client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return false, err
	}
	_, _, _, _, _, _, _, failOverSupported := codec.DecodeClientAuthenticationResponse(resp)
	return failOverSupported, nil
}

func appendHAModeTopologySpreadConstraints(h *hazelcastv1alpha1.Hazelcast) []corev1.TopologySpreadConstraint {
	var topologySpreadConstraints []corev1.TopologySpreadConstraint
	if h.Spec.Scheduling != nil {
		topologySpreadConstraints = append(topologySpreadConstraints, h.Spec.Scheduling.TopologySpreadConstraints...)
	}
	if h.Spec.HighAvailabilityMode != "" {
		switch h.Spec.HighAvailabilityMode {
		case "NODE":
			topologySpreadConstraints = append(topologySpreadConstraints,
				corev1.TopologySpreadConstraint{
					MaxSkew:           1,
					TopologyKey:       "kubernetes.io/hostname",
					WhenUnsatisfiable: corev1.ScheduleAnyway,
					LabelSelector:     &metav1.LabelSelector{MatchLabels: util.Labels(h)},
				})
		case "ZONE":
			topologySpreadConstraints = append(topologySpreadConstraints,
				corev1.TopologySpreadConstraint{
					MaxSkew:           1,
					TopologyKey:       "topology.kubernetes.io/zone",
					WhenUnsatisfiable: corev1.ScheduleAnyway,
					LabelSelector:     &metav1.LabelSelector{MatchLabels: util.Labels(h)},
				})
		}
	}
	return topologySpreadConstraints
}

func env(h *hazelcastv1alpha1.Hazelcast) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  JavaOpts,
			Value: javaOPTS(h),
		},
		{
			Name:  "HZ_PARDOT_ID",
			Value: "operator",
		},
		{
			Name:  "HZ_PHONE_HOME_ENABLED",
			Value: strconv.FormatBool(util.IsPhoneHomeEnabled()),
		},
		{
			Name:  "LOGGING_PATTERN",
			Value: `{"time":"%date{ISO8601}", "level": "%level", "threadName": "%threadName", "logger": "%logger{36}", "msg": "%enc{%m %xEx}{JSON}"}%n`,
		},
		{
			Name:  "LOGGING_LEVEL",
			Value: string(h.Spec.LoggingLevel),
		},
		{
			Name:  "CLASSPATH",
			Value: javaClassPath(h),
		},
	}
	if h.Spec.GetLicenseKeySecretName() != "" {
		envs = append(envs,
			corev1.EnvVar{
				Name: hzLicenseKey,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: h.Spec.GetLicenseKeySecretName(),
						},
						Key: n.LicenseDataKey,
					},
				},
			})
	}

	envs = append(envs, h.Spec.Env...)

	return envs
}

func javaOPTS(h *hazelcastv1alpha1.Hazelcast) string {
	b := strings.Builder{}
	b.WriteString("-Dhazelcast.config=" + path.Join(n.HazelcastMountPath, "hazelcast.yaml"))

	// we should configure JVM to respect container’s resource limits
	b.WriteString(" -XX:+UseContainerSupport")

	jvmMemory := h.Spec.JVM.GetMemory()

	// in addition, we allow user to set explicit memory limits
	if v := jvmMemory.GetInitialRAMPercentage(); v != "" {
		b.WriteString(" -XX:InitialRAMPercentage=" + v)
	}

	if v := jvmMemory.GetMinRAMPercentage(); v != "" {
		b.WriteString(" -XX:MinRAMPercentage=" + v)
	}

	if v := jvmMemory.GetMaxRAMPercentage(); v != "" {
		b.WriteString(" -XX:MaxRAMPercentage=" + v)
	}

	args := h.Spec.JVM.GetArgs()
	if len(args) != 0 {
		args = mergeJVMArgs(args)
	} else {
		for k, v := range DefaultJavaOptions {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}
	}

	for _, a := range args {
		b.WriteString(fmt.Sprintf(" %s", a))
	}

	jvmGC := h.Spec.JVM.GCConfig()

	if jvmGC.IsLoggingEnabled() {
		b.WriteString(" -verbose:gc")
	}

	if v := jvmGC.GetCollector(); v != "" {
		switch v {
		case hazelcastv1alpha1.GCTypeSerial:
			b.WriteString(" -XX:+UseSerialGC")
		case hazelcastv1alpha1.GCTypeParallel:
			b.WriteString(" -XX:+UseParallelGC")
		case hazelcastv1alpha1.GCTypeG1:
			b.WriteString(" -XX:+UseG1GC")
		}
	}

	return b.String()
}

func mergeJVMArgs(args []string) []string {
	configuredArgs := make(map[string]struct{})
	for _, a := range args {
		jvmOptionKeyValue := strings.Split(a, "=")
		configuredArgs[strings.TrimSpace(jvmOptionKeyValue[0])] = struct{}{}
	}

	for k, v := range DefaultJavaOptions {
		_, ok := configuredArgs[k]
		if !ok {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return args
}

func javaClassPath(h *hazelcastv1alpha1.Hazelcast) string {
	b := []string{
		path.Join(n.UserCodeBucketPath, "*"),
		path.Join(n.UserCodeURLPath, "*")}

	if h.Spec.DeprecatedUserCodeDeployment != nil {
		for _, cm := range h.Spec.DeprecatedUserCodeDeployment.RemoteFileConfiguration.ConfigMaps {
			b = append(b, path.Join(n.UserCodeConfigMapPath, cm, "*"))
		}
	}

	return strings.Join(b, ":")
}

func statefulSetAnnotations(sts *appsv1.StatefulSet, h *hazelcastv1alpha1.Hazelcast) map[string]string {
	annotations := make(map[string]string)

	// copy old annotations
	for name, value := range sts.Annotations {
		annotations[name] = value
	}

	// copy user annotations
	for name, value := range h.Spec.Annotations {
		annotations[name] = value
	}

	if !h.Spec.ExposeExternally.IsSmart() {
		return annotations
	}

	// overwrite
	annotations[n.ServicePerPodCountAnnotation] = strconv.Itoa(int(*h.Spec.ClusterSize))

	return annotations
}

func podAnnotations(annotations map[string]string, h *hazelcastv1alpha1.Hazelcast) (map[string]string, error) {
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if h.Spec.ExposeExternally.IsSmart() {
		annotations[n.ExposeExternallyAnnotation] = string(h.Spec.ExposeExternally.MemberAccessType())
	} else {
		delete(annotations, n.ExposeExternallyAnnotation)
	}

	cfg := config.HazelcastWrapper{Hazelcast: configForcingRestart(config.HazelcastBasicConfig(h))}
	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	annotations[n.CurrentHazelcastConfigForcingRestartChecksum] = fmt.Sprint(crc32.ChecksumIEEE(cfgYaml))

	return annotations, nil
}

func hazelcastContainerPorts(h *hazelcastv1alpha1.Hazelcast) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{{
		ContainerPort: n.DefaultHzPort,
		Name:          n.Hazelcast,
		Protocol:      corev1.ProtocolTCP,
	}, {
		ContainerPort: n.MemberServerSocketPort,
		Name:          n.MemberPortName,
		Protocol:      corev1.ProtocolTCP,
	}, {
		ContainerPort: n.RestServerSocketPort,
		Name:          n.RestPortName,
		Protocol:      corev1.ProtocolTCP,
	},
	}

	ports = append(ports, hazelcastContainerWanPorts(h)...)
	return ports
}

func configForcingRestart(hz config.Hazelcast) config.Hazelcast {
	// Apart from these changes, any change in the statefulset spec, labels, annotation can force a restart.
	return config.Hazelcast{
		ClusterName:        hz.ClusterName,
		Jet:                hz.Jet,
		UserCodeDeployment: hz.UserCodeDeployment,
		Properties:         hz.Properties,
		AdvancedNetwork: config.AdvancedNetwork{
			ClientServerSocketEndpointConfig: config.ClientServerSocketEndpointConfig{
				SSL: hz.AdvancedNetwork.ClientServerSocketEndpointConfig.SSL,
			},
			MemberServerSocketEndpointConfig: config.MemberServerSocketEndpointConfig{
				SSL: hz.AdvancedNetwork.MemberServerSocketEndpointConfig.SSL,
			},
		},
		CPSubsystem: hz.CPSubsystem,
	}
}

func metadata(h *hazelcastv1alpha1.Hazelcast) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        h.Name,
		Namespace:   h.Namespace,
		Labels:      labels(h),
		Annotations: h.Spec.Annotations,
	}
}

func labels(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	l := make(map[string]string)

	// copy user labels
	for name, value := range h.Spec.Labels {
		l[name] = value
	}

	// make sure we overwrite user labels
	l[n.ApplicationNameLabel] = n.Hazelcast
	l[n.ApplicationInstanceNameLabel] = h.GetName()
	l[n.ApplicationManagedByLabel] = n.OperatorName

	return l
}

func serviceAccountName(h *hazelcastv1alpha1.Hazelcast) string {
	if h.Spec.ServiceAccountName != "" {
		return h.Spec.ServiceAccountName
	}
	return h.Name
}

func (r *HazelcastReconciler) updateLastSuccessfulConfiguration(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	opResult, err := util.Update(ctx, r.Client, h, func() error {
		controller.InsertLastSuccessfullyAppliedSpec(h.Spec, h)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Hazelcast Annotation", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) updateLastAppliedSpec(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	opResult, err := util.Update(ctx, r.Client, h, func() error {
		controller.InsertLastAppliedSpec(h.Spec, h)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Hazelcast Annotation", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) unmarshalHazelcastSpec(h *hazelcastv1alpha1.Hazelcast, rawLastSpec string) (*hazelcastv1alpha1.HazelcastSpec, error) {
	hs, err := json.Marshal(h.Spec)
	if err != nil {
		err = fmt.Errorf("error marshaling Hazelcast as JSON: %w", err)
		return nil, err
	}
	if rawLastSpec == string(hs) {
		return &h.Spec, nil
	}
	lastSpec := &hazelcastv1alpha1.HazelcastSpec{}
	err = json.Unmarshal([]byte(rawLastSpec), lastSpec)
	if err != nil {
		err = fmt.Errorf("error unmarshaling Last HZ Spec: %w", err)
		return nil, err
	}
	return lastSpec, nil
}

func (r *HazelcastReconciler) detectNewExecutorServices(h *hazelcastv1alpha1.Hazelcast, lastSpec *hazelcastv1alpha1.HazelcastSpec) (map[string]interface{}, error) {
	currentSpec := h.Spec

	existExecutorServices := make(map[string]struct{}, len(lastSpec.ExecutorServices))
	newExecutorServices := make([]hazelcastv1alpha1.ExecutorServiceConfiguration, 0, len(currentSpec.ExecutorServices))
	for _, es := range lastSpec.ExecutorServices {
		existExecutorServices[es.Name] = struct{}{}
	}
	for _, es := range currentSpec.ExecutorServices {
		_, ok := existExecutorServices[es.Name]
		if !ok {
			newExecutorServices = append(newExecutorServices, es)
		}
	}

	existExecutorServices = make(map[string]struct{}, len(lastSpec.DurableExecutorServices))
	newDurableExecutorServices := make([]hazelcastv1alpha1.DurableExecutorServiceConfiguration, 0, len(currentSpec.DurableExecutorServices))
	for _, es := range lastSpec.DurableExecutorServices {
		existExecutorServices[es.Name] = struct{}{}
	}
	for _, es := range currentSpec.DurableExecutorServices {
		_, ok := existExecutorServices[es.Name]
		if !ok {
			newDurableExecutorServices = append(newDurableExecutorServices, es)
		}
	}

	existExecutorServices = make(map[string]struct{}, len(lastSpec.ScheduledExecutorServices))
	newScheduledExecutorServices := make([]hazelcastv1alpha1.ScheduledExecutorServiceConfiguration, 0, len(currentSpec.ScheduledExecutorServices))
	for _, es := range lastSpec.ScheduledExecutorServices {
		existExecutorServices[es.Name] = struct{}{}
	}
	for _, es := range currentSpec.ScheduledExecutorServices {
		_, ok := existExecutorServices[es.Name]
		if !ok {
			newScheduledExecutorServices = append(newScheduledExecutorServices, es)
		}
	}

	return map[string]interface{}{"es": newExecutorServices, "des": newDurableExecutorServices, "ses": newScheduledExecutorServices}, nil
}

func (r *HazelcastReconciler) addExecutorServices(ctx context.Context, client hzclient.Client, newExecutorServices map[string]interface{}) {
	var req *proto.ClientMessage
	for _, es := range newExecutorServices["es"].([]hazelcastv1alpha1.ExecutorServiceConfiguration) {
		esInput := codecTypes.DefaultAddExecutorServiceInput()
		fillAddExecutorServiceInput(esInput, es)
		req = codec.EncodeDynamicConfigAddExecutorConfigRequest(esInput)

		for _, member := range client.OrderedMembers() {
			_, err := client.InvokeOnMember(ctx, req, member.UUID, nil)
			if err != nil {
				continue
			}
		}
	}
	for _, des := range newExecutorServices["des"].([]hazelcastv1alpha1.DurableExecutorServiceConfiguration) {
		esInput := codecTypes.DefaultAddDurableExecutorServiceInput()
		fillAddDurableExecutorServiceInput(esInput, des)
		req = codec.EncodeDynamicConfigAddDurableExecutorConfigRequest(esInput)

		for _, member := range client.OrderedMembers() {
			_, err := client.InvokeOnMember(ctx, req, member.UUID, nil)
			if err != nil {
				continue
			}
		}
	}
	for _, ses := range newExecutorServices["ses"].([]hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) {
		esInput := codecTypes.DefaultAddScheduledExecutorServiceInput()
		fillAddScheduledExecutorServiceInput(esInput, ses)
		req = codec.EncodeDynamicConfigAddScheduledExecutorConfigRequest(esInput)

		for _, member := range client.OrderedMembers() {
			_, err := client.InvokeOnMember(ctx, req, member.UUID, nil)
			if err != nil {
				continue
			}
		}
	}
}

func fillAddExecutorServiceInput(esInput *codecTypes.ExecutorServiceConfig, es hazelcastv1alpha1.ExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.QueueCapacity = es.QueueCapacity
	esInput.UserCodeNamespace = es.UserCodeNamespace
}

func fillAddDurableExecutorServiceInput(esInput *codecTypes.DurableExecutorServiceConfig, es hazelcastv1alpha1.DurableExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.Capacity = es.Capacity
	esInput.Durability = es.Durability
	esInput.UserCodeNamespace = es.UserCodeNamespace
}

func fillAddScheduledExecutorServiceInput(esInput *codecTypes.ScheduledExecutorServiceConfig, es hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.Capacity = es.Capacity
	esInput.CapacityPolicy = es.CapacityPolicy
	esInput.Durability = es.Durability
	esInput.UserCodeNamespace = es.UserCodeNamespace
}
