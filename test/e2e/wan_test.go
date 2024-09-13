package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	proto "github.com/hazelcast/hazelcast-go-client"
	"k8s.io/utils/ptr"

	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast WAN", Group("hz_wan"), func() {
	localPortSrc := strconv.Itoa(9000 + GinkgoParallelProcess())
	localPortTrg := strconv.Itoa(9100 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.WanReplication{}, &hazelcastcomv1alpha1.WanReplicationList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Basic WAN Replication functionality", func() {
		It("successfully replicates data to another cluster", Tag(AnyCloud), func() {
			setLabelAndCRName("hw-1")

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSrcLookupKey.Name: {mapLookupKey.Name},
				hzTrgLookupKey.Name: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating WAN configuration")
			createWanConfig(context.Background(), wanLookupKey, hzCrs[hzTrgLookupKey.Name],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: mapLookupKey.Name},
				}, 1, labels)

			By("checking the size of the maps in the target cluster")
			mapSize := 1024
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPortSrc, mapLookupKey.Name, mapSize)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTrgLookupKey.Name], localPortTrg, mapLookupKey.Name, mapSize, 1*Minute)
		})

		It("maintains replication after source members restart", Tag(AnyCloud), func() {
			setLabelAndCRName("hw-2")

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSrcLookupKey.Name: {mapLookupKey.Name},
				hzTrgLookupKey.Name: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating WAN configuration")
			createWanConfig(context.Background(), wanLookupKey, hzCrs[hzTrgLookupKey.Name],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: mapLookupKey.Name},
				}, 1, labels)

			deletePods(hzSrcLookupKey)
			// Wait for pod to be deleted
			Sleep(5 * Second)
			evaluateReadyMembers(hzSrcLookupKey)

			By("checking the size of the maps in the target cluster")
			mapSize := 1024
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPortSrc, mapLookupKey.Name, mapSize)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTrgLookupKey.Name], localPortTrg, mapLookupKey.Name, mapSize, 1*Minute)
		})
	})

	Context("Handling WAN Replication status", func() {
		It("sets WAN status to 'Failed' when delete Map CR which present as a Map resource in WAN spec", Tag(AnyCloud), func() {
			suffix := setLabelAndCRName("hw-3")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, mapCrs := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map11, Kind: hazelcastcomv1alpha1.ResourceKindMap},
				}, 1, labels)

			Expect(k8sClient.Delete(context.Background(), mapCrs[map11])).Should(BeNil())
			assertObjectDoesNotExist(mapCrs[map11])

			wan = assertWanStatusMapCount(wan, 0)
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusFailed)

			Expect(k8sClient.Delete(context.Background(), wan)).Should(BeNil())
			assertObjectDoesNotExist(wan)
		})

		It("sets WAN status to 'Pending' when delete Map which present as a Hazelcast resource in WAN spec", Tag(Kind|AnyCloud), func() {
			suffix := setLabelAndCRName("hw-4")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, mapCrs := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 1, labels)

			Expect(k8sClient.Delete(context.Background(), mapCrs[map11])).Should(BeNil())
			assertObjectDoesNotExist(mapCrs[map11])
			wan = assertWanStatusMapCount(wan, 0)
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusPending)

			Expect(k8sClient.Delete(context.Background(), wan)).Should(BeNil())
			assertObjectDoesNotExist(wan)
		})

		It("sets WAN status to 'Success' when resource map is created after the WAN CR", Tag(Kind|AnyCloud), func() {
			suffix := setLabelAndCRName("hw-5")

			// Hazelcast CRs
			hzSource := "hz-source" + suffix
			hzTarget := "hz-target" + suffix

			hzSourceCr := hazelcastconfig.Default(types.NamespacedName{
				Name:      hzSource,
				Namespace: hzSrcLookupKey.Namespace,
			}, labels)
			hzSourceCr.Spec.ClusterName = hzSource
			hzSourceCr.Spec.ClusterSize = ptr.To(int32(1))
			CreateHazelcastCRWithoutCheck(hzSourceCr)
			evaluateReadyMembers(types.NamespacedName{Name: hzSource, Namespace: hzSrcLookupKey.Namespace})

			hzTargetCr := hazelcastconfig.Default(types.NamespacedName{
				Name:      hzTarget,
				Namespace: hzSrcLookupKey.Namespace,
			}, labels)
			hzTargetCr.Spec.ClusterName = hzTarget
			hzTargetCr.Spec.ClusterSize = ptr.To(int32(1))
			CreateHazelcastCRWithoutCheck(hzTargetCr)
			evaluateReadyMembers(types.NamespacedName{Name: hzTarget, Namespace: hzTrgLookupKey.Namespace})

			// Map CRs
			mapBeforeWan := "map0" + suffix
			mapAfterWan := "map1" + suffix

			// Creating the map before the WanReplication CR
			mapBeforeWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapBeforeWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
			Expect(k8sClient.Create(context.Background(), mapBeforeWanCr)).Should(Succeed())
			assertMapStatus(mapBeforeWanCr, hazelcastcomv1alpha1.MapSuccess)

			By("creating WAN configuration")
			wan := hazelcastconfig.WanReplication(
				wanLookupKey,
				hzTargetCr.Spec.ClusterName,
				hzclient.HazelcastUrl(hzTargetCr),
				[]hazelcastcomv1alpha1.ResourceSpec{
					{
						Name: mapBeforeWan,
						Kind: hazelcastcomv1alpha1.ResourceKindMap,
					},
					{
						Name: mapAfterWan,
						Kind: hazelcastcomv1alpha1.ResourceKindMap,
					},
				},
				labels,
			)
			Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusFailed)

			// Creating the map after the WanReplication CR
			mapAfetWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapAfterWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
			Expect(k8sClient.Create(context.Background(), mapAfetWanCr)).Should(Succeed())
			assertMapStatus(mapAfetWanCr, hazelcastcomv1alpha1.MapSuccess)

			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
			assertWanStatusMapCount(wan, 2)
		})

		It("updated the member status", Tag(AnyCloud), func() {
			setLabelAndCRName("hw-6")

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSrcLookupKey.Name: {mapLookupKey.Name},
				hzTrgLookupKey.Name: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating WAN configuration")
			wan := createWanConfig(context.Background(), wanLookupKey, hzCrs[hzTrgLookupKey.Name],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: mapLookupKey.Name},
				}, 1, labels)

			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)

			By("checking the size of the maps in the target cluster")
			mapSize := 1024
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPortSrc, mapLookupKey.Name, mapSize)

			By("checking the wan member status")
			Sleep(Second * 10)
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      wan.Name,
				Namespace: wan.Namespace,
			}, wan)
			Expect(err).ToNot(HaveOccurred())

			Expect(wan.Status.WanReplicationMapsStatus).Should(HaveLen(1))
			var mapStatus hazelcastcomv1alpha1.WanReplicationMapStatus
			for k := range wan.Status.WanReplicationMapsStatus {
				mapStatus = wan.Status.WanReplicationMapsStatus[k]
			}
			Expect(mapStatus.MembersStatus).Should(HaveLen(1))
			var wanMemberStatus hazelcastcomv1alpha1.WanMemberStatus
			for k := range mapStatus.MembersStatus {
				wanMemberStatus = mapStatus.MembersStatus[k]
			}
			Expect(wanMemberStatus.IsConnected).Should(BeTrue())
			Expect(wanMemberStatus.State).Should(BeEquivalentTo("REPLICATING"))
		})
	})

	Context("Updating WAN configuration", func() {
		It("initially fails after removal of replicated Hazelcast CR, then succeeds after removal it from the WAN spec", Tag(AnyCloud), func() {
			suffix := setLabelAndCRName("hw-7")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12 := "map12" + suffix
			map13 := "map13" + suffix

			hzSource2 := "hz2-source" + suffix
			map21 := "map21" + suffix
			map22 := "map22" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11, map12, map13},
				hzSource2: {map21, map22},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
					{Name: hzSource2, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 5, labels)

			Expect(k8sClient.Delete(context.Background(), hzCrs[hzSource2])).Should(BeNil())
			assertObjectDoesNotExist(hzCrs[hzSource2])
			wan = assertWanStatusMapCount(wan, 3)
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusFailed)

			wan.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
				{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
			}
			Expect(k8sClient.Update(context.Background(), wan)).Should(BeNil())
			wan = assertWanStatusMapCount(wan, 3)
			_ = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
		})

		It("stops replication for maps removed from WAN spec", Tag(Kind|AnyCloud), func() {
			suffix := setLabelAndCRName("hw-8")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12 := "map12" + suffix

			hzSource2 := "hz2-source" + suffix
			map21 := "map21" + suffix
			map22 := "map22" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11, map12},
				hzSource2: {map21, map22},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
					{Name: map21, Kind: hazelcastcomv1alpha1.ResourceKindMap},
					{Name: map22, Kind: hazelcastcomv1alpha1.ResourceKindMap},
				}, 4, labels)

			By("filling the maps in the source clusters")
			entryCount := 10
			portForwardingDo(hzCrs[hzSource1], localPortSrc, func(cl *proto.Client) {
				fillMapData(context.TODO(), cl, map11, entryCount)
				fillMapData(context.TODO(), cl, map12, entryCount)
			})
			portForwardingDo(hzCrs[hzSource2], localPortSrc, func(cl *proto.Client) {
				fillMapData(context.TODO(), cl, map21, entryCount)
				fillMapData(context.TODO(), cl, map22, entryCount)
			})

			By("checking the size of the maps in the target cluster")
			portForwardingDo(hzCrs[hzTarget1], localPortTrg, func(cl *proto.Client) {
				waitForMapSize(context.TODO(), cl, map11, entryCount, 1*Minute)
				waitForMapSize(context.TODO(), cl, map12, entryCount, 1*Minute)
				waitForMapSize(context.TODO(), cl, map21, entryCount, 1*Minute)
				waitForMapSize(context.TODO(), cl, map22, entryCount, 1*Minute)
			})

			By("stopping replicating all maps but map22")

			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
			wan.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
				{Name: map22, Kind: hazelcastcomv1alpha1.ResourceKindMap},
			}
			Expect(k8sClient.Update(context.Background(), wan)).Should(BeNil())
			wan = assertWanStatusMapCount(wan, 1)
			_ = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)

			currentSize := entryCount
			newEntryCount := 50
			portForwardingDo(hzCrs[hzSource1], localPortSrc, func(cl *proto.Client) {
				fillMapData(context.TODO(), cl, map11, newEntryCount)
				fillMapData(context.TODO(), cl, map12, newEntryCount)
			})
			portForwardingDo(hzCrs[hzSource2], localPortSrc, func(cl *proto.Client) {
				fillMapData(context.TODO(), cl, map21, newEntryCount)
				fillMapData(context.TODO(), cl, map22, newEntryCount)
			})

			By("checking the size of the maps in the target cluster")
			portForwardingDo(hzCrs[hzTarget1], localPortTrg, func(cl *proto.Client) {
				waitForMapSize(context.TODO(), cl, map11, entryCount, 1*Minute)
				waitForMapSize(context.TODO(), cl, map12, entryCount, 1*Minute)
				waitForMapSize(context.TODO(), cl, map21, entryCount, 1*Minute)
				waitForMapSize(context.TODO(), cl, map22, currentSize+newEntryCount, 1*Minute)
			})
		})

		It("continues replication when 1 of 2 maps references is deleted from WAN spec", Tag(Kind|AnyCloud), func() {
			suffix := setLabelAndCRName("hw-9")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12 := "map12" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11, map12},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map11, Kind: hazelcastcomv1alpha1.ResourceKindMap},
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 2, labels)

			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
			wan.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
				{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
			}
			Expect(k8sClient.Update(context.Background(), wan)).Should(BeNil())
			Sleep(5 * Second)
			wan = assertWanStatusMapCount(wan, 2)
			_ = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
		})

		It("verifies replication initiation for maps added after WAN setup", Tag(AnyCloud), func() {
			suffix := setLabelAndCRName("hw-10")

			// Hazelcast CRs
			hzSource := "hz-source" + suffix
			hzTarget := "hz-target" + suffix

			hzSourceCr := hazelcastconfig.Default(types.NamespacedName{
				Name:      hzSource,
				Namespace: hzSrcLookupKey.Namespace,
			}, labels)
			hzSourceCr.Spec.ClusterName = hzSource
			hzSourceCr.Spec.ClusterSize = ptr.To(int32(1))
			CreateHazelcastCRWithoutCheck(hzSourceCr)
			evaluateReadyMembers(types.NamespacedName{Name: hzSource, Namespace: hzSrcLookupKey.Namespace})

			hzTargetCr := hazelcastconfig.Default(types.NamespacedName{
				Name:      hzTarget,
				Namespace: hzSrcLookupKey.Namespace,
			}, labels)
			hzTargetCr.Spec.ClusterName = hzTarget
			hzTargetCr.Spec.ClusterSize = ptr.To(int32(1))
			CreateHazelcastCRWithoutCheck(hzTargetCr)
			evaluateReadyMembers(types.NamespacedName{Name: hzTarget, Namespace: hzTrgLookupKey.Namespace})

			// Map CRs
			mapBeforeWan := "map0" + suffix
			mapAfterWan := "map1" + suffix

			// Creating the map before the WanReplication CR
			mapBeforeWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapBeforeWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
			Expect(k8sClient.Create(context.Background(), mapBeforeWanCr)).Should(Succeed())
			assertMapStatus(mapBeforeWanCr, hazelcastcomv1alpha1.MapSuccess)

			By("creating WAN configuration")
			wan := hazelcastconfig.WanReplication(
				wanLookupKey,
				hzTargetCr.Spec.ClusterName,
				fmt.Sprintf("%s.%s.svc.cluster.local:%d", hzTargetCr.Name, hzTargetCr.Namespace, n.WanDefaultPort),
				[]hazelcastcomv1alpha1.ResourceSpec{
					{
						Name: hzSource,
						Kind: hazelcastcomv1alpha1.ResourceKindHZ,
					},
				},
				labels,
			)
			Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)

			assertWanStatusMapCount(wan, 1)

			mapSize := 1024

			By("checking the size of the map created before the wan")
			fillTheMapDataPortForward(context.Background(), hzSourceCr, localPortSrc, mapBeforeWan, mapSize)
			waitForMapSizePortForward(context.Background(), hzTargetCr, localPortSrc, mapBeforeWan, mapSize, Minute)

			// Creating the map after the WanReplication CR
			mapAfterWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapAfterWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
			Expect(k8sClient.Create(context.Background(), mapAfterWanCr)).Should(Succeed())
			assertMapStatus(mapAfterWanCr, hazelcastcomv1alpha1.MapSuccess)

			assertWanStatusMapCount(wan, 2)

			By("checking the size of the map created after the wan")
			fillTheMapDataPortForward(context.Background(), hzSourceCr, localPortSrc, mapAfterWan, mapSize)
			waitForMapSizePortForward(context.Background(), hzTargetCr, localPortSrc, mapAfterWan, mapSize, Minute)
		})

		It("handles different map names for target cluster replication", Tag(AnyCloud), func() {
			suffix := setLabelAndCRName("hw-11")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12CrName := "map12" + suffix
			map12MapName := map12CrName + "-mapName"

			hzSource2 := "hz2-source" + suffix
			map21 := "map21" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11},
				hzSource2: {map21},
				hzTarget1: nil,
			}, hzNamespace, labels)
			// Create the map with CRName != mapName
			createMapCRWithMapName(context.Background(), map12CrName, map12MapName, types.NamespacedName{Name: hzSource1, Namespace: hzNamespace})

			By("creating first WAN configuration")
			createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map12CrName, Kind: hazelcastcomv1alpha1.ResourceKindMap},
					{Name: hzSource2, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 2, labels)

			By("creating second WAN configuration")
			createWanConfig(context.Background(), types.NamespacedName{Name: "wan2" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map11},
				}, 1, labels)

			By("filling the maps in the source clusters")
			mapSize := 10
			portForwardingDo(hzCrs[hzSource1], localPortSrc, func(cl *proto.Client) {
				fillMapData(context.TODO(), cl, map11, mapSize)
				fillMapData(context.TODO(), cl, map12MapName, mapSize)
			})
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource2], localPortSrc, map21, mapSize)

			By("checking the size of the maps in the target cluster")
			portForwardingDo(hzCrs[hzTarget1], localPortTrg, func(cl *proto.Client) {
				waitForMapSize(context.TODO(), cl, map11, mapSize, 1*Minute)
				waitForMapSize(context.TODO(), cl, map12MapName, mapSize, 1*Minute)
				waitForMapSize(context.TODO(), cl, map21, mapSize, 1*Minute)
			})
		})
	})

})
