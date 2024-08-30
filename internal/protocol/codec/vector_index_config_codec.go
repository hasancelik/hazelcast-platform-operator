/*
* Copyright (c) 2008-2024, Hazelcast, Inc. All Rights Reserved.
*
* Licensed under the Apache License, Version 2.0 (the "License")
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
* http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package codec

import (
	proto "github.com/hazelcast/hazelcast-go-client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	VectorIndexConfigCodecMetricFieldOffset                = 0
	VectorIndexConfigCodecDimensionFieldOffset             = VectorIndexConfigCodecMetricFieldOffset + proto.IntSizeInBytes
	VectorIndexConfigCodecMaxDegreeFieldOffset             = VectorIndexConfigCodecDimensionFieldOffset + proto.IntSizeInBytes
	VectorIndexConfigCodecEfConstructionFieldOffset        = VectorIndexConfigCodecMaxDegreeFieldOffset + proto.IntSizeInBytes
	VectorIndexConfigCodecUseDeduplicationFieldOffset      = VectorIndexConfigCodecEfConstructionFieldOffset + proto.IntSizeInBytes
	VectorIndexConfigCodecUseDeduplicationInitialFrameSize = VectorIndexConfigCodecUseDeduplicationFieldOffset + proto.BooleanSizeInBytes
)

func EncodeVectorIndexConfig(clientMessage *proto.ClientMessage, vectorIndexConfig types.VectorIndexConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, VectorIndexConfigCodecUseDeduplicationInitialFrameSize))
	EncodeInt(initialFrame.Content, VectorIndexConfigCodecMetricFieldOffset, vectorIndexConfig.Metric)
	EncodeInt(initialFrame.Content, VectorIndexConfigCodecDimensionFieldOffset, vectorIndexConfig.Dimension)
	EncodeInt(initialFrame.Content, VectorIndexConfigCodecMaxDegreeFieldOffset, vectorIndexConfig.MaxDegree)
	EncodeInt(initialFrame.Content, VectorIndexConfigCodecEfConstructionFieldOffset, vectorIndexConfig.EfConstruction)
	EncodeBoolean(initialFrame.Content, VectorIndexConfigCodecUseDeduplicationFieldOffset, vectorIndexConfig.UseDeduplication)
	clientMessage.AddFrame(initialFrame)

	EncodeNullableForString(clientMessage, vectorIndexConfig.Name)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func EncodeListMultiFrameForVectorIndexConfig(message *proto.ClientMessage, indexes []types.VectorIndexConfig) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(indexes); i++ {
		EncodeVectorIndexConfig(message, indexes[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}

func DecodeVectorIndexConfig(frameIterator *proto.ForwardFrameIterator) types.VectorIndexConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	metric := DecodeInt(initialFrame.Content, VectorIndexConfigCodecMetricFieldOffset)
	dimension := DecodeInt(initialFrame.Content, VectorIndexConfigCodecDimensionFieldOffset)
	maxDegree := DecodeInt(initialFrame.Content, VectorIndexConfigCodecMaxDegreeFieldOffset)
	efConstruction := DecodeInt(initialFrame.Content, VectorIndexConfigCodecEfConstructionFieldOffset)
	useDeduplication := DecodeBoolean(initialFrame.Content, VectorIndexConfigCodecUseDeduplicationFieldOffset)

	name := DecodeNullableForString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.VectorIndexConfig{
		Name:             name,
		Metric:           metric,
		Dimension:        dimension,
		MaxDegree:        maxDegree,
		EfConstruction:   efConstruction,
		UseDeduplication: useDeduplication,
	}
}
