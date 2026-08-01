/*
 * Simple caching library with expiration capabilities
 *     Copyright (c) 2013-2017, Christian Muehlhaeuser <muesli@gmail.com>
 *
 *   For license see LICENSE.txt
 */

package cache2go

import (
	"bytes"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	k = "testkey"
	v = "testvalue"
)

func TestCache(t *testing.T) {
	// add an expiring item after a non-expiring one to
	// trigger expirationCheck iterating over non-expiring items
	table := Cache("testCache")
	table.Add(k+"_1", 0*time.Second, v)
	table.Add(k+"_2", 1*time.Second, v)

	// check if both items are still there
	p, err := table.Value(k + "_1")
	if err != nil || p == nil || p.Data().(string) != v {
		t.Error("Error retrieving non expiring data from cache", err)
	}
	p, err = table.Value(k + "_2")
	if err != nil || p == nil || p.Data().(string) != v {
		t.Error("Error retrieving data from cache", err)
	}

	// sanity checks
	if p.AccessCount() != 1 {
		t.Error("Error getting correct access count")
	}
	if p.LifeSpan() != 1*time.Second {
		t.Error("Error getting correct life-span")
	}
	if p.AccessedOn().Unix() == 0 {
		t.Error("Error getting access time")
	}
	if p.CreatedOn().Unix() == 0 {
		t.Error("Error getting creation time")
	}
}

func TestCacheExpire(t *testing.T) {
	table := Cache("testCache")

	table.Add(k+"_1", 250*time.Millisecond, v+"_1")
	table.Add(k+"_2", 200*time.Millisecond, v+"_2")

	time.Sleep(100 * time.Millisecond)

	// check key `1` is still alive
	_, err := table.Value(k + "_1")
	if err != nil {
		t.Error("Error retrieving value from cache:", err)
	}

	time.Sleep(150 * time.Millisecond)

	// check key `1` again, it should still be alive since we just accessed it
	_, err = table.Value(k + "_1")
	if err != nil {
		t.Error("Error retrieving value from cache:", err)
	}

	// check key `2`, it should have been removed by now
	_, err = table.Value(k + "_2")
	if err == nil {
		t.Error("Found key which should have been expired by now")
	}
}

func TestExists(t *testing.T) {
	// add an expiring item
	table := Cache("testExists")
	table.Add(k, 0, v)
	// check if it exists
	if !table.Exists(k) {
		t.Error("Error verifying existing data in cache")
	}
}

func TestNotFoundAdd(t *testing.T) {
	table := Cache("testNotFoundAdd")

	if !table.NotFoundAdd(k, 0, v) {
		t.Error("Error verifying NotFoundAdd, data not in cache")
	}

	if table.NotFoundAdd(k, 0, v) {
		t.Error("Error verifying NotFoundAdd data in cache")
	}
}

func TestNotFoundAddConcurrency(t *testing.T) {
	table := Cache("testNotFoundAdd")

	var finish sync.WaitGroup
	var added int32
	var idle int32

	fn := func(id int) {
		for i := 0; i < 100; i++ {
			if table.NotFoundAdd(i, 0, i+id) {
				atomic.AddInt32(&added, 1)
			} else {
				atomic.AddInt32(&idle, 1)
			}
			time.Sleep(0)
		}
		finish.Done()
	}

	finish.Add(10)
	go fn(0x0000)
	go fn(0x1100)
	go fn(0x2200)
	go fn(0x3300)
	go fn(0x4400)
	go fn(0x5500)
	go fn(0x6600)
	go fn(0x7700)
	go fn(0x8800)
	go fn(0x9900)
	finish.Wait()

	t.Log(added, idle)

	table.Foreach(func(key interface{}, item *CacheItem) {
		v, _ := item.Data().(int)
		k, _ := key.(int)
		t.Logf("%02x  %04x\n", k, v)
	})
}

func TestCacheKeepAlive(t *testing.T) {
	// add an expiring item
	table := Cache("testKeepAlive")
	p := table.Add(k, 250*time.Millisecond, v)

	// keep it alive before it expires
	time.Sleep(100 * time.Millisecond)
	p.KeepAlive()

	// check it's still alive after it was initially supposed to expire
	time.Sleep(150 * time.Millisecond)
	if !table.Exists(k) {
		t.Error("Error keeping item alive")
	}

	// check it expires eventually
	time.Sleep(300 * time.Millisecond)
	if table.Exists(k) {
		t.Error("Error expiring item after keeping it alive")
	}
}

func TestDelete(t *testing.T) {
	// add an item to the cache
	table := Cache("testDelete")
	table.Add(k, 0, v)
	// check it's really cached
	p, err := table.Value(k)
	if err != nil || p == nil || p.Data().(string) != v {
		t.Error("Error retrieving data from cache", err)
	}
	// try to delete it
	table.Delete(k)
	// verify it has been deleted
	p, err = table.Value(k)
	if err == nil || p != nil {
		t.Error("Error deleting data")
	}

	// test error handling
	_, err = table.Delete(k)
	if err == nil {
		t.Error("Expected error deleting item")
	}
}

func TestFlush(t *testing.T) {
	// add an item to the cache
	table := Cache("testFlush")
	table.Add(k, 10*time.Second, v)
	// flush the entire table
	table.Flush()

	// try to retrieve the item
	p, err := table.Value(k)
	if err == nil || p != nil {
		t.Error("Error flushing table")
	}
	// make sure there's really nothing else left in the cache
	if table.Count() != 0 {
		t.Error("Error verifying count of flushed table")
	}
}

func TestCount(t *testing.T) {
	// add a huge amount of items to the cache
	table := Cache("testCount")
	count := 100000
	for i := 0; i < count; i++ {
		key := k + strconv.Itoa(i)
		table.Add(key, 10*time.Second, v)
	}
	// confirm every single item has been cached
	for i := 0; i < count; i++ {
		key := k + strconv.Itoa(i)
		p, err := table.Value(key)
		if err != nil || p == nil || p.Data().(string) != v {
			t.Error("Error retrieving data")
		}
	}
	// make sure the item count matches (no dupes etc.)
	if table.Count() != count {
		t.Error("Data count mismatch")
	}
}

func TestDataLoader(t *testing.T) {
	// setup a cache with a configured data-loader
	table := Cache("testDataLoader")
	table.SetDataLoader(func(key interface{}, args ...interface{}) *CacheItem {
		var item *CacheItem
		if key.(string) != "nil" {
			val := k + key.(string)
			i := NewCacheItem(key, 500*time.Millisecond, val)
			item = i
		}

		return item
	})

	// make sure data-loader works as expected and handles unloadable keys
	_, err := table.Value("nil")
	if err == nil || table.Exists("nil") {
		t.Error("Error validating data loader for nil values")
	}

	// retrieve a bunch of items via the data-loader
	for i := 0; i < 10; i++ {
		key := k + strconv.Itoa(i)
		vp := k + key
		p, err := table.Value(key)
		if err != nil || p == nil || p.Data().(string) != vp {
			t.Error("Error validating data loader")
		}
	}
}

func TestAccessCount(t *testing.T) {
	// add 100 items to the cache
	count := 100
	table := Cache("testAccessCount")
	for i := 0; i < count; i++ {
		table.Add(i, 10*time.Second, v)
	}
	// never access the first item, access the second item once, the third
	// twice and so on...
	for i := 0; i < count; i++ {
		for j := 0; j < i; j++ {
			table.Value(i)
		}
	}

	// check MostAccessed returns the items in correct order
	ma := table.MostAccessed(int64(count))
	for i, item := range ma {
		if item.Key() != count-1-i {
			t.Error("Most accessed items seem to be sorted incorrectly")
		}
	}

	// check MostAccessed returns the correct amount of items
	ma = table.MostAccessed(int64(count - 1))
	if len(ma) != count-1 {
		t.Error("MostAccessed returns incorrect amount of items")
	}
}

func TestCallbacks(t *testing.T) {
	var m sync.Mutex
	addedKey := ""
	removedKey := ""
	calledAddedItem := false
	calledRemoveItem := false
	expired := false
	calledExpired := false

	// setup a cache with AddedItem & SetAboutToDelete handlers configured
	table := Cache("testCallbacks")
	table.SetAddedItemCallback(func(item *CacheItem) {
		m.Lock()
		addedKey = item.Key().(string)
		m.Unlock()
	})
	table.SetAddedItemCallback(func(item *CacheItem) {
		m.Lock()
		calledAddedItem = true
		m.Unlock()
	})
	table.SetAboutToDeleteItemCallback(func(item *CacheItem) {
		m.Lock()
		removedKey = item.Key().(string)
		m.Unlock()
	})

	table.SetAboutToDeleteItemCallback(func(item *CacheItem) {
		m.Lock()
		calledRemoveItem = true
		m.Unlock()
	})
	// add an item to the cache and setup its AboutToExpire handler
	i := table.Add(k, 500*time.Millisecond, v)
	i.SetAboutToExpireCallback(func(key interface{}) {
		m.Lock()
		expired = true
		m.Unlock()
	})

	i.SetAboutToExpireCallback(func(key interface{}) {
		m.Lock()
		calledExpired = true
		m.Unlock()
	})

	// verify the AddedItem handler works
	time.Sleep(250 * time.Millisecond)
	m.Lock()
	if addedKey == k && !calledAddedItem {
		t.Error("AddedItem callback not working")
	}
	m.Unlock()
	// verify the AboutToDelete handler works
	time.Sleep(500 * time.Millisecond)
	m.Lock()
	if removedKey == k && !calledRemoveItem {
		t.Error("AboutToDeleteItem callback not working:" + k + "_" + removedKey)
	}
	// verify the AboutToExpire handler works
	if expired && !calledExpired {
		t.Error("AboutToExpire callback not working")
	}
	m.Unlock()

}

func TestCallbackQueue(t *testing.T) {
	var m sync.Mutex
	addedKey := ""
	addedkeyCallback2 := ""
	secondCallbackResult := "second"
	removedKey := ""
	removedKeyCallback := ""
	expired := false
	calledExpired := false
	// setup a cache with AddedItem & SetAboutToDelete handlers configured
	table := Cache("testCallbacks")

	// test callback queue
	table.AddAddedItemCallback(func(item *CacheItem) {
		m.Lock()
		addedKey = item.Key().(string)
		m.Unlock()
	})
	table.AddAddedItemCallback(func(item *CacheItem) {
		m.Lock()
		addedkeyCallback2 = secondCallbackResult
		m.Unlock()
	})

	table.AddAboutToDeleteItemCallback(func(item *CacheItem) {
		m.Lock()
		removedKey = item.Key().(string)
		m.Unlock()
	})
	table.AddAboutToDeleteItemCallback(func(item *CacheItem) {
		m.Lock()
		removedKeyCallback = secondCallbackResult
		m.Unlock()
	})

	i := table.Add(k, 500*time.Millisecond, v)
	i.AddAboutToExpireCallback(func(key interface{}) {
		m.Lock()
		expired = true
		m.Unlock()
	})
	i.AddAboutToExpireCallback(func(key interface{}) {
		m.Lock()
		calledExpired = true
		m.Unlock()
	})

	time.Sleep(250 * time.Millisecond)
	m.Lock()
	if addedKey != k && addedkeyCallback2 != secondCallbackResult {
		t.Error("AddedItem callback queue not working")
	}
	m.Unlock()

	time.Sleep(500 * time.Millisecond)
	m.Lock()
	if removedKey != k && removedKeyCallback != secondCallbackResult {
		t.Error("Item removed callback queue not working")
	}
	m.Unlock()

	// test removing of the callbacks
	table.RemoveAddedItemCallbacks()
	table.RemoveAboutToDeleteItemCallback()
	secondItemKey := "itemKey02"
	expired = false
	i = table.Add(secondItemKey, 500*time.Millisecond, v)
	i.SetAboutToExpireCallback(func(key interface{}) {
		m.Lock()
		expired = true
		m.Unlock()
	})
	i.RemoveAboutToExpireCallback()

	// verify if the callbacks were removed
	time.Sleep(250 * time.Millisecond)
	m.Lock()
	if addedKey == secondItemKey {
		t.Error("AddedItemCallbacks were not removed")
	}
	m.Unlock()

	// verify the AboutToDelete handler works
	time.Sleep(500 * time.Millisecond)
	m.Lock()
	if removedKey == secondItemKey {
		t.Error("AboutToDeleteItem not removed")
	}
	// verify the AboutToExpire handler works
	if !expired && !calledExpired {
		t.Error("AboutToExpire callback not working")
	}
	m.Unlock()
}

func TestLogger(t *testing.T) {
	// setup a logger
	out := new(bytes.Buffer)
	l := log.New(out, "cache2go ", log.Ldate|log.Ltime)

	// setup a cache with this logger
	table := Cache("testLogger")
	table.SetLogger(l)
	table.Add(k, 0, v)

	time.Sleep(100 * time.Millisecond)

	// verify the logger has been used
	if out.Len() == 0 {
		t.Error("Logger is empty")
	}
}
