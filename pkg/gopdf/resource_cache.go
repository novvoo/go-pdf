package gopdf

import (
	"sync"
	"time"
)

// ResourceCache 资源缓存管理器
type ResourceCache struct {
	mu         sync.RWMutex
	resources  map[string]*CachedResource
	maxSize    int
	ttl        time.Duration
	hits       int64
	misses     int64
}

// CachedResource 缓存的资源
type CachedResource struct {
	Data      interface{}
	CreatedAt time.Time
	LastUsed  time.Time
	Size      int
}

// NewResourceCache 创建新的资源缓存
func NewResourceCache(maxSize int, ttl time.Duration) *ResourceCache {
	cache := &ResourceCache{
		resources: make(map[string]*CachedResource),
		maxSize:   maxSize,
		ttl:       ttl,
	}
	
	// 启动清理 goroutine
	if ttl > 0 {
		go cache.cleanupLoop()
	}
	
	return cache
}

// Get 获取缓存的资源
func (c *ResourceCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	resource, exists := c.resources[key]
	if !exists {
		c.misses++
		return nil, false
	}
	
	// 检查是否过期
	if c.ttl > 0 && time.Since(resource.CreatedAt) > c.ttl {
		c.misses++
		return nil, false
	}
	
	resource.LastUsed = time.Now()
	c.hits++
	return resource.Data, true
}

// Set 设置缓存资源
func (c *ResourceCache) Set(key string, data interface{}, size int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 检查缓存大小限制
	if c.maxSize > 0 && len(c.resources) >= c.maxSize {
		c.evictOldest()
	}
	
	c.resources[key] = &CachedResource{
		Data:      data,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Size:      size,
	}
}

// Delete 删除缓存资源
func (c *ResourceCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.resources, key)
}

// Clear 清空缓存
func (c *ResourceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resources = make(map[string]*CachedResource)
	c.hits = 0
	c.misses = 0
}

// Stats 获取缓存统计信息
func (c *ResourceCache) Stats() (hits, misses int64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, len(c.resources)
}

// evictOldest 驱逐最旧的资源（LRU）
func (c *ResourceCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, resource := range c.resources {
		if oldestKey == "" || resource.LastUsed.Before(oldestTime) {
			oldestKey = key
			oldestTime = resource.LastUsed
		}
	}
	
	if oldestKey != "" {
		delete(c.resources, oldestKey)
	}
}

// cleanupLoop 定期清理过期资源
func (c *ResourceCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()
	
	for range ticker.C {
		c.cleanup()
	}
}

// cleanup 清理过期资源
func (c *ResourceCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for key, resource := range c.resources {
		if now.Sub(resource.CreatedAt) > c.ttl {
			delete(c.resources, key)
		}
	}
}

// 全局资源缓存实例
var (
	globalResourceCache *ResourceCache
	cacheOnce           sync.Once
)

// GetGlobalResourceCache 获取全局资源缓存
func GetGlobalResourceCache() *ResourceCache {
	cacheOnce.Do(func() {
		// 默认缓存 1000 个资源，TTL 为 5 分钟
		globalResourceCache = NewResourceCache(1000, 5*time.Minute)
	})
	return globalResourceCache
}
