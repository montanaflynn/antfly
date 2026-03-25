package vectorindex

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"sync/atomic"

	"github.com/cockroachdb/pebble/v2"
)

type HBCDebugSearchProfile struct {
	TotalNS              uint64
	RootLoadNS           uint64
	NodeCacheMissNS      uint64
	NodeCacheMisses      uint64
	QuantizedCacheMissNS uint64
	QuantizedCacheMisses uint64
	ChildExpandNS        uint64
	LeafScoreNS          uint64
	ApproxFillPushes     uint64
	ApproxRejects        uint64
	ApproxCloserPushes   uint64
	ApproxMaybePushes    uint64
	ApproxDefinitePops   uint64
	ApproxTrimPops       uint64
	NodesVisited         uint64
	LeavesExplored       uint64
	ApproxNodesExpanded  uint64
	ApproxLeavesScored   uint64
	ApproxVectorsScored  uint64
	ExactVectorsScored   uint64
	RerankedVectors      uint64
	RerankVectorLoadNS   uint64
	RerankDistanceNS     uint64
}

var lastHBCDebugTotalNS atomic.Uint64
var lastHBCDebugRootLoadNS atomic.Uint64
var lastHBCDebugNodeCacheMissNS atomic.Uint64
var lastHBCDebugNodeCacheMisses atomic.Uint64
var lastHBCDebugQuantizedCacheMissNS atomic.Uint64
var lastHBCDebugQuantizedCacheMisses atomic.Uint64
var lastHBCDebugChildExpandNS atomic.Uint64
var lastHBCDebugLeafScoreNS atomic.Uint64
var lastHBCDebugApproxFillPushes atomic.Uint64
var lastHBCDebugApproxRejects atomic.Uint64
var lastHBCDebugApproxCloserPushes atomic.Uint64
var lastHBCDebugApproxMaybePushes atomic.Uint64
var lastHBCDebugApproxDefinitePops atomic.Uint64
var lastHBCDebugApproxTrimPops atomic.Uint64
var lastHBCDebugNodesVisited atomic.Uint64
var lastHBCDebugLeavesExplored atomic.Uint64
var lastHBCDebugApproxNodesExpanded atomic.Uint64
var lastHBCDebugApproxLeavesScored atomic.Uint64
var lastHBCDebugApproxVectorsScored atomic.Uint64
var lastHBCDebugExactVectorsScored atomic.Uint64
var lastHBCDebugRerankedVectors atomic.Uint64
var lastHBCDebugRerankVectorLoadNS atomic.Uint64
var lastHBCDebugRerankDistanceNS atomic.Uint64

var hbcDebugNodeCacheMissNS atomic.Uint64
var hbcDebugNodeCacheMisses atomic.Uint64
var hbcDebugQuantizedCacheMissNS atomic.Uint64
var hbcDebugQuantizedCacheMisses atomic.Uint64

func recordHBCDebugSearchProfile(profile HBCDebugSearchProfile) {
	lastHBCDebugTotalNS.Store(profile.TotalNS)
	lastHBCDebugRootLoadNS.Store(profile.RootLoadNS)
	lastHBCDebugNodeCacheMissNS.Store(profile.NodeCacheMissNS)
	lastHBCDebugNodeCacheMisses.Store(profile.NodeCacheMisses)
	lastHBCDebugQuantizedCacheMissNS.Store(profile.QuantizedCacheMissNS)
	lastHBCDebugQuantizedCacheMisses.Store(profile.QuantizedCacheMisses)
	lastHBCDebugChildExpandNS.Store(profile.ChildExpandNS)
	lastHBCDebugLeafScoreNS.Store(profile.LeafScoreNS)
	lastHBCDebugApproxFillPushes.Store(profile.ApproxFillPushes)
	lastHBCDebugApproxRejects.Store(profile.ApproxRejects)
	lastHBCDebugApproxCloserPushes.Store(profile.ApproxCloserPushes)
	lastHBCDebugApproxMaybePushes.Store(profile.ApproxMaybePushes)
	lastHBCDebugApproxDefinitePops.Store(profile.ApproxDefinitePops)
	lastHBCDebugApproxTrimPops.Store(profile.ApproxTrimPops)
	lastHBCDebugNodesVisited.Store(profile.NodesVisited)
	lastHBCDebugLeavesExplored.Store(profile.LeavesExplored)
	lastHBCDebugApproxNodesExpanded.Store(profile.ApproxNodesExpanded)
	lastHBCDebugApproxLeavesScored.Store(profile.ApproxLeavesScored)
	lastHBCDebugApproxVectorsScored.Store(profile.ApproxVectorsScored)
	lastHBCDebugExactVectorsScored.Store(profile.ExactVectorsScored)
	lastHBCDebugRerankedVectors.Store(profile.RerankedVectors)
	lastHBCDebugRerankVectorLoadNS.Store(profile.RerankVectorLoadNS)
	lastHBCDebugRerankDistanceNS.Store(profile.RerankDistanceNS)
}

func LastHBCDebugSearchProfile() HBCDebugSearchProfile {
	return HBCDebugSearchProfile{
		TotalNS:              lastHBCDebugTotalNS.Load(),
		RootLoadNS:           lastHBCDebugRootLoadNS.Load(),
		NodeCacheMissNS:      lastHBCDebugNodeCacheMissNS.Load(),
		NodeCacheMisses:      lastHBCDebugNodeCacheMisses.Load(),
		QuantizedCacheMissNS: lastHBCDebugQuantizedCacheMissNS.Load(),
		QuantizedCacheMisses: lastHBCDebugQuantizedCacheMisses.Load(),
		ChildExpandNS:        lastHBCDebugChildExpandNS.Load(),
		LeafScoreNS:          lastHBCDebugLeafScoreNS.Load(),
		ApproxFillPushes:     lastHBCDebugApproxFillPushes.Load(),
		ApproxRejects:        lastHBCDebugApproxRejects.Load(),
		ApproxCloserPushes:   lastHBCDebugApproxCloserPushes.Load(),
		ApproxMaybePushes:    lastHBCDebugApproxMaybePushes.Load(),
		ApproxDefinitePops:   lastHBCDebugApproxDefinitePops.Load(),
		ApproxTrimPops:       lastHBCDebugApproxTrimPops.Load(),
		NodesVisited:         lastHBCDebugNodesVisited.Load(),
		LeavesExplored:       lastHBCDebugLeavesExplored.Load(),
		ApproxNodesExpanded:  lastHBCDebugApproxNodesExpanded.Load(),
		ApproxLeavesScored:   lastHBCDebugApproxLeavesScored.Load(),
		ApproxVectorsScored:  lastHBCDebugApproxVectorsScored.Load(),
		ExactVectorsScored:   lastHBCDebugExactVectorsScored.Load(),
		RerankedVectors:      lastHBCDebugRerankedVectors.Load(),
		RerankVectorLoadNS:   lastHBCDebugRerankVectorLoadNS.Load(),
		RerankDistanceNS:     lastHBCDebugRerankDistanceNS.Load(),
	}
}

type HBCDebugNode struct {
	ID       uint64    `json:"id"`
	IsLeaf   bool      `json:"is_leaf"`
	Parent   uint64    `json:"parent"`
	Level    int       `json:"level"`
	Centroid []float32 `json:"centroid"`
	Children []uint64  `json:"children,omitempty"`
	Members  []uint64  `json:"members,omitempty"`
}

type HBCDebugDump struct {
	Dimension       uint32         `json:"dimension"`
	BranchingFactor int            `json:"branching_factor"`
	LeafSize        int            `json:"leaf_size"`
	SearchWidth     int            `json:"search_width"`
	UseQuantization bool           `json:"use_quantization"`
	UseROT          bool           `json:"use_rot"`
	QuantizerSeed   uint64         `json:"quantizer_seed"`
	RootNode        uint64         `json:"root_node"`
	ActiveCount     uint64         `json:"active_count"`
	NodeCount       uint64         `json:"node_count"`
	Nodes           []HBCDebugNode `json:"nodes"`
}

func (idx *HBCIndex) DebugDump() (*HBCDebugDump, error) {
	meta, err := idx.getMetadata(idx.indexDB)
	if err != nil {
		return nil, err
	}

	nodes := make([]HBCDebugNode, 0, meta.NodeCount)
	seen := map[uint64]struct{}{}
	queue := []uint64{meta.RootNode}

	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if nodeID == 0 {
			continue
		}
		if _, ok := seen[nodeID]; ok {
			continue
		}
		seen[nodeID] = struct{}{}

		node, err := idx.loadNode(idx.indexDB, nil, nodeID)
		if err != nil {
			return nil, err
		}

		debugNode := HBCDebugNode{
			ID:       node.ID,
			IsLeaf:   node.IsLeaf,
			Parent:   node.Parent,
			Level:    node.Level,
			Centroid: slices.Clone(node.Centroid),
			Children: slices.Clone(node.Children),
			Members:  slices.Clone(node.Members),
		}
		nodes = append(nodes, debugNode)
		queue = append(queue, node.Children...)
	}

	slices.SortFunc(nodes, func(a, b HBCDebugNode) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})

	return &HBCDebugDump{
		Dimension:       idx.config.Dimension,
		BranchingFactor: idx.config.BranchingFactor,
		LeafSize:        idx.config.LeafSize,
		SearchWidth:     idx.config.SearchWidth,
		UseQuantization: idx.config.UseQuantization,
		UseROT:          idx.config.UseRandomOrthoTrans,
		QuantizerSeed:   idx.config.QuantizerSeed,
		RootNode:        meta.RootNode,
		ActiveCount:     meta.ActiveCount,
		NodeCount:       meta.NodeCount,
		Nodes:           nodes,
	}, nil
}

func OpenDebugPebbleBackedHBC(baseDir string, config HBCConfig, randSource rand.Source) (*HBCIndex, func() error, error) {
	db, err := pebble.Open(filepath.Join(baseDir, "pebble"), &pebble.Options{})
	if err != nil {
		return nil, nil, err
	}
	config.VectorDB = db
	config.IndexDB = db

	idx, err := NewHBCIndex(config, randSource)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	closeFn := func() error {
		if err := idx.Close(); err != nil {
			_ = db.Close()
			return err
		}
		return db.Close()
	}
	return idx, closeFn, nil
}

func OpenDebugPebbleBackedHBCWithBatch(baseDir string, config HBCConfig, randSource rand.Source, batch *Batch) (*HBCIndex, func() error, error) {
	db, err := pebble.Open(filepath.Join(baseDir, "pebble"), &pebble.Options{})
	if err != nil {
		return nil, nil, err
	}

	if batch != nil {
		if err := populateDebugVectors(db, config.Name, batch); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
	}

	config.VectorDB = db
	config.IndexDB = db

	idx, err := NewHBCIndex(config, randSource)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	closeFn := func() error {
		if err := idx.Close(); err != nil {
			_ = db.Close()
			return err
		}
		return db.Close()
	}
	return idx, closeFn, nil
}

func populateDebugVectors(db *pebble.DB, indexName string, batch *Batch) error {
	suffix := fmt.Appendf(nil, ":i:%s:e", indexName)
	pbatch := db.NewBatch()
	defer func() { _ = pbatch.Close() }()

	if batch.MetadataList == nil {
		batch.MetadataList = make([][]byte, len(batch.IDs))
	}

	for i, id := range batch.IDs {
		docID := batch.MetadataList[i]
		if docID == nil {
			docID = fmt.Appendf(nil, "doc_%d", id)
			batch.MetadataList[i] = docID
		}
		vecKey := append(bytes.Clone(docID), suffix...)
		vecData := make([]byte, 0, 8+4*(len(batch.Vectors[i])+1))
		vecData, err := EncodeEmbeddingWithHashID(vecData, batch.Vectors[i], 0)
		if err != nil {
			return err
		}
		if err := pbatch.Set(vecKey, vecData, nil); err != nil {
			return err
		}
	}

	return pbatch.Commit(pebble.Sync)
}
