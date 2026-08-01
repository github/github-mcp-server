package main

import (
	"fmt"
	"github.com/muesli/cache2go"
	"strconv"
)

func main() {
	cache := cache2go.Cache("myCache")

	// The data loader gets called automatically whenever something
	// tries to retrieve a non-existing key from the cache.
	cache.SetDataLoader(func(key interface{}, args ...interface{}) *cache2go.CacheItem {
		// Apply some clever loading logic here, e.g. read values for
		// this key from database, network or file.
		val := "This is a test with key " + key.(string)

		// This helper method creates the cached item for us. Yay!
		item := cache2go.NewCacheItem(key, 0, val)
		return item
	})

	// Let's retrieve a few auto-generated items from the cache.
	for i := 0; i < 10; i++ {
		res, err := cache.Value("someKey_" + strconv.Itoa(i))
		if err == nil {
			fmt.Println("Found value in cache:", res.Data())
		} else {
			fmt.Println("Error retrieving value from cache:", err)
		}
	}
}
