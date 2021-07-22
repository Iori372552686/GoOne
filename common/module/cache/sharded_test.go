package cache

import (
	insecurerand "math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var shardedKeys = []string{
	"f",
	"fo",
	"foo",
	"barf",
	"barfo",
	"foobar",
	"bazbarf",
	"bazbarfo",
	"bazbarfoo",
	"foobarbazq",
	"foobarbazqu",
	"foobarbazquu",
	"foobarbazquux",
}

func TestShardedCache(t *testing.T) {
	tc := NewSharded(DefaultExpiration, 0, 13)
	for _, v := range shardedKeys {
		tc.Set(v, "value", DefaultExpiration)
	}
}

func TestShardedCache_GetOrStore(t *testing.T) {
	tc := NewSharded(DefaultExpiration, 0, 13)
	rv, err := tc.GetOrStore("foo", func() (interface{}, error) {
		return "bar", nil
	}, DefaultExpiration)
	require.NoError(t, err)
	require.Equal(t, "bar", rv)

	rv, err = tc.GetOrStore("bar", func() (interface{}, error) {
		return nil, errors.New("oops")
	}, DefaultExpiration)
	require.Error(t, err)
	require.Nil(t, rv)

	_, exists := tc.Get("bar")
	require.False(t, exists)
}

func TestShardedCache_ItemsCount(t *testing.T) {
	tc := NewSharded(DefaultExpiration, 0, 13)

	tc.Set("foo1", "1", DefaultExpiration)
	tc.Set("bar2", "2", DefaultExpiration)
	tc.Set("baz3", "3", DefaultExpiration)

	require.Equal(t, 3, tc.ItemsCount())
}

func TestShardedCache_Get(t *testing.T) {
	tc := NewSharded(DefaultExpiration, 0, 13)
	_, exists := tc.Get("foo")
	require.False(t, exists)

	tc.Set("foo", "bar", DefaultExpiration)
	v, exists := tc.Get("foo")
	require.True(t, exists)
	require.Equal(t, "bar", v)

}

func BenchmarkShardedCacheGetExpiring(b *testing.B) {
	benchmarkShardedCacheGet(b, 5*time.Minute)
}

func BenchmarkShardedCacheGetNotExpiring(b *testing.B) {
	benchmarkShardedCacheGet(b, NoExpiration)
}

func benchmarkShardedCacheGet(b *testing.B, exp time.Duration) {
	b.StopTimer()
	tc := NewSharded(exp, 0, 10)
	tc.Set("foobarba", "zquux", DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Get("foobarba")
	}
}

func BenchmarkShardedCacheGetManyConcurrentExpiring(b *testing.B) {
	benchmarkShardedCacheGetManyConcurrent(b, 5*time.Minute)
}

func BenchmarkShardedCacheGetManyConcurrentNotExpiring(b *testing.B) {
	benchmarkShardedCacheGetManyConcurrent(b, NoExpiration)
}

func benchmarkShardedCacheGetManyConcurrent(b *testing.B, exp time.Duration) {
	b.StopTimer()
	n := 10000
	tsc := NewSharded(exp, 0, 20)
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		k := "foo" + strconv.Itoa(n)
		keys[i] = k
		tsc.Set(k, "bar", DefaultExpiration)
	}
	each := b.N / n
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for _, v := range keys {
		go func(k string) {
			for j := 0; j < each; j++ {
				tsc.Get(k)
			}
			wg.Done()
		}(v)
	}
	b.StartTimer()
	wg.Wait()
}

func BenchmarkDjb33(b *testing.B) {
	s := insecurerand.Uint32()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		djb33(s, "foobar")
	}
}

func BenchmarkDjb2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		djb2([]byte("foobar"))
	}
}
