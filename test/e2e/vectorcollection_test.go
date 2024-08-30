package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Vector Collection Config", Group("vector_collection"), func() {
	localPort := strconv.Itoa(8000 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.VectorCollection{}, &hazelcastcomv1alpha1.VectorCollectionList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Creating Vector Collection configurations", func() {
		It("should successfully create a Vector Collection config", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("vc-1")
			hazelcast := hazelcastconfig.Default(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)

			By("creating the default vector collection config")
			vc := hazelcastconfig.VectorCollection(vcLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), vc)).Should(Succeed())
			vc = assertDataStructureStatus(vcLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.VectorCollection{}).(*hazelcastcomv1alpha1.VectorCollection)

			By("checking if the vectorCollection config is created correctly")
			memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
			println(memberConfigXML)
			vectorCollectionConfig := getVectorCollectionConfigFromMemberConfig(memberConfigXML, vc.GetDSName())
			Expect(vectorCollectionConfig).NotTo(BeNil())

			Expect(vectorCollectionConfig.Indexes.Index).Should(HaveLen(1))
			Expect(vectorCollectionConfig.Indexes.Index[0].Name).Should(Equal(vc.Spec.Indexes[0].Name))
			Expect(vectorCollectionConfig.Indexes.Index[0].Dimension).Should(Equal(vc.Spec.Indexes[0].Dimension))
			Expect(vectorCollectionConfig.Indexes.Index[0].MaxDegree).Should(Equal(vc.Spec.Indexes[0].MaxDegree))
			Expect(vectorCollectionConfig.Indexes.Index[0].EfConstruction).Should(Equal(vc.Spec.Indexes[0].EfConstruction))
			Expect(vectorCollectionConfig.Indexes.Index[0].UseDeduplication).Should(Equal(vc.Spec.Indexes[0].UseDeduplication))
		})
	})
})
