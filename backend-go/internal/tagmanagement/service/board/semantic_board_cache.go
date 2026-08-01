package board

import (
	"sync"
	"time"
)

const configTTL = 5 * time.Minute

type boardCache struct {
	mu sync.RWMutex

	// Board auxiliaries cache — invalidated on board composition change
	boardAuxiliaries    []BoardAuxiliaryLabel
	boardAuxiliariesSet bool

	// Board embeddings cache — invalidated with auxiliaries
	boardEmbeddings    map[uint][]float64
	boardEmbeddingsSet bool

	// AI config cache — TTL-based
	config     *SemanticBoardMatchConfig
	configTime time.Time
}

func (c *boardCache) GetBoardAuxiliaries() ([]BoardAuxiliaryLabel, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.boardAuxiliariesSet {
		return nil, false
	}
	return c.boardAuxiliaries, true
}

func (c *boardCache) SetBoardAuxiliaries(data []BoardAuxiliaryLabel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boardAuxiliaries = data
	c.boardAuxiliariesSet = true
}

func (c *boardCache) GetBoardEmbeddings() (map[uint][]float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.boardEmbeddingsSet {
		return nil, false
	}
	return c.boardEmbeddings, true
}

func (c *boardCache) SetBoardEmbeddings(data map[uint][]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boardEmbeddings = data
	c.boardEmbeddingsSet = true
}

func (c *boardCache) GetConfig() (*SemanticBoardMatchConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.config == nil || time.Since(c.configTime) >= configTTL {
		return nil, false
	}
	return c.config, true
}

func (c *boardCache) SetConfig(config SemanticBoardMatchConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = &config
	c.configTime = time.Now()
}

// packageBoardCache is the shared board cache instance used by both
// SemanticBoardMatchingService and SemanticBoardUpgradeService.
var packageBoardCache = &boardCache{}

func (c *boardCache) InvalidateBoardData() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boardAuxiliaries = nil
	c.boardAuxiliariesSet = false
	c.boardEmbeddings = nil
	c.boardEmbeddingsSet = false
}

// InvalidateConfig forces the next LoadConfig call to read from the database.
func (c *boardCache) InvalidateConfig() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = nil
	c.configTime = time.Time{}
}

// InvalidateMatchingConfigCache clears the cached matching config so the next
// LoadConfig call reads fresh values from the database.
func InvalidateMatchingConfigCache() {
	packageBoardCache.InvalidateConfig()
}

// InvalidateBoardCache clears the package-level board cache (board data + config).
// Tests must call this after ResetTestData truncates semantic_labels / board tables,
// since ResetTestData clears the DB but not this in-memory cache (stale entries
// would otherwise span test boundaries and break backfill/matching assertions).
func InvalidateBoardCache() {
	packageBoardCache.InvalidateBoardData()
	packageBoardCache.InvalidateConfig()
}
