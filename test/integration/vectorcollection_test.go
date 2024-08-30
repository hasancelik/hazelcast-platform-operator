package integration

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("VectorCollection CR", func() {
	const namespace = "default"

	vectorCollectionOf := func(vectorSpec hazelcastv1alpha1.VectorCollectionSpec) *hazelcastv1alpha1.VectorCollection {
		cs, _ := json.Marshal(vectorSpec)
		return &hazelcastv1alpha1.VectorCollection{
			ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(cs)),
			Spec:       vectorSpec,
		}
	}

	BeforeEach(func() {
		By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
		licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
		assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.VectorCollection{}, &hazelcastv1alpha1.VectorCollectionList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("spec validation", func() {
		It("should not allow special characters in the name", func() {
			vc := vectorCollectionOf(hazelcastv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: "hazelcast",
				Name:                  "not-allo#ed",
				Indexes: []hazelcastv1alpha1.VectorIndex{
					{
						Dimension:        10,
						Metric:           hazelcastv1alpha1.Dot,
						MaxDegree:        10,
						EfConstruction:   10,
						UseDeduplication: true,
					},
				},
			})
			Expect(k8sClient.Create(context.Background(), vc)).Should(
				MatchError(ContainSubstring("Invalid value: \"not-allo#ed\": spec.name in body should match '^[a-zA-Z0-9_\\-*]*$'")))
		})
		It("should allow _-* characters in the name", func() {
			vc := vectorCollectionOf(hazelcastv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: "hazelcast",
				Name:                  "is-allo*ed_",
				Indexes: []hazelcastv1alpha1.VectorIndex{
					{
						Dimension:        10,
						Metric:           hazelcastv1alpha1.Dot,
						MaxDegree:        10,
						EfConstruction:   10,
						UseDeduplication: true,
					},
				},
			})
			Expect(k8sClient.Create(context.Background(), vc)).Should(Succeed())
		})
		It("should not allow special characters in the index name", func() {
			vc := vectorCollectionOf(hazelcastv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: "hazelcast",
				Name:                  "vector-name",
				Indexes: []hazelcastv1alpha1.VectorIndex{
					{
						Name:             "not-allo*ed",
						Dimension:        10,
						Metric:           hazelcastv1alpha1.Dot,
						MaxDegree:        10,
						EfConstruction:   10,
						UseDeduplication: true,
					},
				},
			})
			Expect(k8sClient.Create(context.Background(), vc)).Should(
				MatchError(ContainSubstring("Invalid value: \"not-allo*ed\": spec.indexes[0].name in body should match '^[a-zA-Z0-9_\\-]*$'")))
		})
		It("should allow _- characters in the name", func() {
			vc := vectorCollectionOf(hazelcastv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: "hazelcast",
				Name:                  "vector-name",
				Indexes: []hazelcastv1alpha1.VectorIndex{
					{
						Name:             "is-allowed_",
						Dimension:        10,
						Metric:           hazelcastv1alpha1.Dot,
						MaxDegree:        10,
						EfConstruction:   10,
						UseDeduplication: true,
					},
				},
			})
			Expect(k8sClient.Create(context.Background(), vc)).Should(Succeed())
		})
		It("should not allow empty name for multi-index collection", func() {
			vc := vectorCollectionOf(hazelcastv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: "hazelcast",
				Name:                  "vector-name",
				Indexes: []hazelcastv1alpha1.VectorIndex{
					{
						Name:             "is-allowed_",
						Dimension:        10,
						Metric:           hazelcastv1alpha1.Dot,
						MaxDegree:        10,
						EfConstruction:   10,
						UseDeduplication: true,
					},
					{
						Dimension:        10,
						Metric:           hazelcastv1alpha1.Dot,
						MaxDegree:        10,
						EfConstruction:   10,
						UseDeduplication: true,
					},
				},
			})
			Expect(k8sClient.Create(context.Background(), vc)).Should(
				MatchError(ContainSubstring("spec.indexes[1].name: Required value: must be set for multi-index collection")))
		})
		It("should not allow no indexes in collection", func() {
			vc := vectorCollectionOf(hazelcastv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: "hazelcast",
				Name:                  "vector-name",
				Indexes:               []hazelcastv1alpha1.VectorIndex{},
			})
			Expect(k8sClient.Create(context.Background(), vc)).Should(
				MatchError(ContainSubstring("Invalid value: 0: spec.indexes in body should have at least 1 items")))
		})
		It("should not allow updating config", func() {
			vc := vectorCollectionOf(hazelcastv1alpha1.VectorCollectionSpec{
				HazelcastResourceName: "hazelcast",
				Name:                  "vector-name",
				Indexes: []hazelcastv1alpha1.VectorIndex{
					{
						Dimension:        10,
						Metric:           hazelcastv1alpha1.Dot,
						MaxDegree:        10,
						EfConstruction:   10,
						UseDeduplication: true,
					},
				},
			})

			By("creating VectorCollection CR successfully")
			Expect(k8sClient.Create(context.Background(), vc)).Should(Succeed())

			By("trying to update dimension")

			var err error
			for {
				Expect(k8sClient.Get(
					context.Background(), types.NamespacedName{Namespace: vc.Namespace, Name: vc.Name}, vc)).Should(Succeed())
				vc.Spec.Indexes[0].Dimension = 20
				err = k8sClient.Update(context.Background(), vc)
				if errors.IsConflict(err) {
					continue
				}
				break
			}
			Expect(err).Should(MatchError(ContainSubstring("Forbidden: cannot be updated")))
		})
	})
})
