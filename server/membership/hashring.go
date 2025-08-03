package membership

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"

	"github.com/iworkflowio/ipubsub/service/log"
	"github.com/iworkflowio/ipubsub/service/log/tag"
)

type Hashring interface {
	GetNodeForStreamId(streamId string, membershipVersion int64, allNodes []NodeInfo) (NodeInfo, error)
}

type hashringImpl struct {
	logger            log.Logger
	allNodes          []NodeInfo
	virtualNodes      int
	membershipVersion int64
	ring              map[uint64]NodeInfo // hash -> node mapping
	sortedHashes      []uint64            // sorted hash values for binary search
	mu                sync.RWMutex        // protect the ring data structures
}

func NewHashring(logger log.Logger, virtualNodes int) Hashring {
	return &hashringImpl{
		logger:            logger,
		virtualNodes:      virtualNodes,
		membershipVersion: 0,
		ring:              make(map[uint64]NodeInfo),
		sortedHashes:      make([]uint64, 0),
	}
}

func (h *hashringImpl) GetNodeForStreamId(streamId string, membershipVersion int64, allNodes []NodeInfo) (NodeInfo, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update the ring if membership has changed
	if membershipVersion > h.membershipVersion {
		h.membershipVersion = membershipVersion
		h.allNodes = allNodes
		err := h.buildRing()
		if err != nil {
			return NodeInfo{}, fmt.Errorf("failed to build hash ring: %w", err)
		}
	}

	// If no nodes available, return error
	if len(h.allNodes) == 0 {
		return NodeInfo{}, fmt.Errorf("no nodes available in the cluster")
	}

	// If only one node, return it directly
	if len(h.allNodes) == 1 {
		return h.allNodes[0], nil
	}

	// Hash the streamId
	streamHash := h.hash(streamId)

	// Find the first node on or after this hash
	nodeHash := h.findNode(streamHash)
	node, exists := h.ring[nodeHash]
	if !exists {
		return NodeInfo{}, fmt.Errorf("internal error: node not found in ring for hash %d", nodeHash)
	}

	return node, nil
}

// buildRing constructs the consistent hash ring with virtual nodes
func (h *hashringImpl) buildRing() error {
	// Clear existing ring
	h.ring = make(map[uint64]NodeInfo)
	h.sortedHashes = make([]uint64, 0)

	// Add virtual nodes for each real node
	for _, node := range h.allNodes {
		for i := 0; i < h.virtualNodes; i++ {
			// Create a unique virtual node identifier
			virtualNodeKey := fmt.Sprintf("%s:%s:%d:%d", node.Name, node.Addr, node.Port, i)
			hash := h.hash(virtualNodeKey)

			// Add to ring
			h.ring[hash] = node
			h.sortedHashes = append(h.sortedHashes, hash)
		}
	}

	// Sort the hashes for binary search
	sort.Slice(h.sortedHashes, func(i, j int) bool {
		return h.sortedHashes[i] < h.sortedHashes[j]
	})

	h.logger.Info("Hash ring built",
		tag.Value(len(h.allNodes)),
		tag.Value(h.virtualNodes),
		tag.Value(len(h.sortedHashes)))

	return nil
}

// findNode finds the first node on or after the given hash using binary search
func (h *hashringImpl) findNode(targetHash uint64) uint64 {
	if len(h.sortedHashes) == 0 {
		return 0
	}

	// Binary search for the first hash >= targetHash
	idx := sort.Search(len(h.sortedHashes), func(i int) bool {
		return h.sortedHashes[i] >= targetHash
	})

	// If no hash >= targetHash found, wrap around to the first hash
	if idx == len(h.sortedHashes) {
		idx = 0
	}

	return h.sortedHashes[idx]
}

// hash creates a 64-bit hash of the input string using SHA-256
func (h *hashringImpl) hash(key string) uint64 {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hashBytes := hasher.Sum(nil)

	// Use the first 8 bytes of the hash as a uint64
	return binary.BigEndian.Uint64(hashBytes[:8])
}
