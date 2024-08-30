package types

type VectorCollectionConfigs struct {
	VectorCollections []VectorCollectionInput `xml:"vector-collection"`
}

type VectorCollectionInput struct {
	Name    string  `xml:"name,attr"`
	Indexes Indexes `xml:"indexes"`
}

type Indexes struct {
	Index []VectorIndexConfig `xml:"index"`
}

type VectorIndexConfig struct {
	Name             string `xml:"name,attr,omitempty"`
	Metric           int32
	Dimension        int32 `xml:"dimension"`
	MaxDegree        int32 `xml:"max-degree"`
	EfConstruction   int32 `xml:"ef-construction"`
	UseDeduplication bool  `xml:"use-deduplication"`
}
