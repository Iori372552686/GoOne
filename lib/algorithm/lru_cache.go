package algorithm

/// LRU淘汰算法的cache, 线程安全...吗？？（可能有bug）

import (
	"container/list"
	"errors"
	"sync"
)

type CacheNode struct {
	Key, Value interface{}
}

func (node *CacheNode) NewCacheNode(k, v interface{}) *CacheNode {
	return &CacheNode{k, v}
}

type LRUCache struct {
	Capacity int32
	dropList *list.List
	cacheMap map[interface{}]*list.Element

	listLock 	sync.RWMutex
	mapLock 	sync.RWMutex
}

func NewLRUCache(cap int32) *LRUCache {
	return &LRUCache{
		Capacity: cap,
		dropList: list.New(),
		cacheMap: make(map[interface{}]*list.Element),
	}
}

func (lru *LRUCache) Size() int {
	lru.listLock.RLock()
	defer lru.listLock.RUnlock()

	return lru.dropList.Len()
}

func (lru *LRUCache) Set(k, v interface{}) error {
	if lru.dropList == nil {
		return errors.New("LRUCache not init.")
	}

	lru.listLock.Lock()
	lru.mapLock.Lock()
	defer lru.listLock.Unlock()
	defer lru.mapLock.Unlock()

	if pElement, ok := lru.cacheMap[k]; ok {
		lru.dropList.MoveToFront(pElement)
		pElement.Value.(*CacheNode).Value = v
		return nil
	}

	newElement := lru.dropList.PushFront(&CacheNode{k, v})
	lru.cacheMap[k] = newElement

	if int32(lru.dropList.Len()) > lru.Capacity {
		//移掉最后一个
		lastElement := lru.dropList.Back()
		if lastElement == nil {
			return nil
		}
		cacheNode := lastElement.Value.(*CacheNode)
		delete(lru.cacheMap,cacheNode.Key)
		lru.dropList.Remove(lastElement)
	}
	return nil
}


func (lru *LRUCache) Get(k interface{}) (v interface{}, ret bool, err error) {
	if lru.cacheMap == nil {
		return v, false, errors.New("LRUCache not init.")
	}

	lru.mapLock.RLock()
	defer lru.mapLock.RUnlock()
	if pElement,ok := lru.cacheMap[k]; ok {
		lru.listLock.Lock()
		lru.dropList.MoveToFront(pElement)
		lru.listLock.Unlock()
		return pElement.Value.(*CacheNode).Value, true, nil
	}
	return v, false, nil
}


func (lru *LRUCache) Remove(k interface{}) bool {
	if lru.cacheMap == nil {
		return false
	}

	lru.listLock.Lock()
	lru.mapLock.Lock()
	defer lru.listLock.Unlock()
	defer lru.mapLock.Unlock()

	if pElement,ok := lru.cacheMap[k]; ok {
		cacheNode := pElement.Value.(*CacheNode)
		delete(lru.cacheMap, cacheNode.Key)
		lru.dropList.Remove(pElement)
		return true
	}
	return false
}
