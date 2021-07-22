package cache

import (
	"crypto/rand"
	"math"
	"math/big"
	insecurerand "math/rand"
	"os"
	"runtime"
	"time"
)

type ShardedCache struct {
	*shardedCache
}

type shardedCache struct {
	seed    uint32
	m       uint32
	cs      []*cache
	janitor *shardedJanitor
}

// djb2 with better shuffling. 5x faster than FNV with the hash.Hash overhead.
func djb33(seed uint32, k string) uint32 {
	var (
		l = uint32(len(k))
		d = 5381 + seed + l
		i = uint32(0)
	)
	// Why is all this 5x faster than a for loop?
	if l >= 4 {
		for i < l-4 {
			d = (d * 33) ^ uint32(k[i])
			d = (d * 33) ^ uint32(k[i+1])
			d = (d * 33) ^ uint32(k[i+2])
			d = (d * 33) ^ uint32(k[i+3])
			i += 4
		}
	}
	switch l - i {
	case 1:
	case 2:
		d = (d * 33) ^ uint32(k[i])
	case 3:
		d = (d * 33) ^ uint32(k[i])
		d = (d * 33) ^ uint32(k[i+1])
	case 4:
		d = (d * 33) ^ uint32(k[i])
		d = (d * 33) ^ uint32(k[i+1])
		d = (d * 33) ^ uint32(k[i+2])
	}
	return d ^ (d >> 16)
}

// Source: http://www.cse.yorku.ca/~oz/hash.html
func djb2(b []byte) int {
	var hash = 5381
	for i := range b {
		hash = ((hash << 5) + hash) + int(b[i])
	}
	return hash & 0x7FFFFFFF
}

func (sc *shardedCache) bucket(k string) *cache {
	return sc.cs[djb33(sc.seed, k)%sc.m]
}

// Store an item in the cache.
func (sc *shardedCache) Set(k string, x interface{}, d time.Duration) {
	sc.bucket(k).Set(k, x, d)
}

// Store an item in the cache if the key does not exist.
func (sc *shardedCache) Add(k string, x interface{}, d time.Duration) error {
	return sc.bucket(k).Add(k, x, d)
}

// Replace an item in the cache if the key exist.
func (sc *shardedCache) Replace(k string, x interface{}, d time.Duration) error {
	return sc.bucket(k).Replace(k, x, d)
}

// Retrieve an item from the cache by key.
func (sc *shardedCache) Get(k string) (interface{}, bool) {
	return sc.bucket(k).Get(k)
}

// Determine if an item exists in the cache.
func (sc *shardedCache) Has(k string) bool {
	return sc.bucket(k).Has(k)
}

// Retrieve an item from the cache and delete it.
func (sc *shardedCache) Pop(k string) (rv interface{}, deleted bool) {
	return sc.bucket(k).Pop(k)
}

// Set a new expiration on an item, returns true on success or false on failure
func (sc *shardedCache) Touch(k string, d time.Duration) bool {
	return sc.bucket(k).Touch(k, d)
}

// Get an item from the cache, or store the value.
func (sc *shardedCache) GetOrStore(
	k string,
	v func() (interface{}, error),
	d time.Duration,
) (interface{}, error) {
	value, found := sc.bucket(k).Get(k)
	if !found {
		value, err := v()
		if err != nil {
			return nil, err
		}
		sc.bucket(k).Set(k, value, d)
		return value, nil
	}
	return value, nil
}

// Remove an item from the cache.
func (sc *shardedCache) Delete(k string) {
	sc.bucket(k).Delete(k)
}

// Remove all expired items from the cache.
func (sc *shardedCache) DeleteExpired() {
	for _, v := range sc.cs {
		v.DeleteExpired()
	}
}

// Returns the items in the cache. This may include items that have expired,
// but have not yet been cleaned up. If this is significant, the Expiration
// fields of the items should be checked. Note that explicit synchronization
// is needed to use a cache and its corresponding Items() return values at
// the same time, as the maps are shared.
func (sc *shardedCache) Items() []map[string]Item {
	res := make([]map[string]Item, len(sc.cs))
	for i, v := range sc.cs {
		res[i] = v.Items()
	}
	return res
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (sc *shardedCache) ItemsCount() (rv int) {
	for _, v := range sc.cs {
		rv = rv + v.ItemCount()
	}
	return
}

// Delete all items from the cache.
func (sc *shardedCache) Flush() {
	for _, v := range sc.cs {
		v.Flush()
	}
}

type shardedJanitor struct {
	Interval time.Duration
	// https://dave.cheney.net/2014/03/25/the-empty-struct
	stop chan struct{}
}

func (j *shardedJanitor) Run(sc *shardedCache) {
	j.stop = make(chan struct{})
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.DeleteExpired()
		case <-j.stop:
			return
		}
	}
}

func stopShardedJanitor(sc *ShardedCache) {
	sc.janitor.stop <- struct{}{}
}

func runShardedJanitor(sc *shardedCache, ci time.Duration) {
	j := &shardedJanitor{
		Interval: ci,
	}
	sc.janitor = j
	go j.Run(sc)
}

func newShardedCache(n int, de time.Duration) *shardedCache {
	max := big.NewInt(0).SetUint64(uint64(math.MaxUint32))
	rnd, err := rand.Int(rand.Reader, max)
	var seed uint32
	if err != nil {
		os.Stderr.Write([]byte("WARNING: bian-cache's newShardedCache failed to read from the system CSPRNG " +
			"(/dev/urandom or equivalent.) Your system's security may be compromised." +
			" Continuing with an insecure seed.\n"))
		seed = insecurerand.Uint32()
	} else {
		seed = uint32(rnd.Uint64())
	}
	sc := &shardedCache{
		seed: seed,
		m:    uint32(n),
		cs:   make([]*cache, n),
	}
	for i := 0; i < n; i++ {
		c := &cache{
			defaultExpiration: de,
			items:             map[string]Item{},
		}
		sc.cs[i] = c
	}
	return sc
}

func NewSharded(defaultExpiration, cleanupInterval time.Duration, shards int) *ShardedCache {
	if defaultExpiration == 0 {
		defaultExpiration = -1
	}
	sc := newShardedCache(shards, defaultExpiration)
	rv := &ShardedCache{sc}
	if cleanupInterval > 0 {
		runShardedJanitor(sc, cleanupInterval)
		runtime.SetFinalizer(rv, stopShardedJanitor)
	}
	return rv
}
