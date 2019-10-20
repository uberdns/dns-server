package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/go-redis/redis"
)

// CacheControlMessage -- struct for storing/parsing redis cache control messages
//  					  from the api server
type CacheControlMessage struct {
	Action string
	Type   string
	Object string
}

func cacheMessageHandler(msg CacheControlMessage) error {
	switch strings.ToLower(msg.Type) {
	case "record":
		var record Record
		json.Unmarshal([]byte(msg.Object), &record)

		//Record cache manage routes
		switch strings.ToLower(msg.Action) {
		case "create":
			addRecordToCache(record)
		case "purge":
			removeRecordFromCache(record)
		}
	case "domain":
		var domain Domain
		json.Unmarshal([]byte(msg.Object), &domain)

		//Domain cache manage routes
		switch strings.ToLower(msg.Action) {
		case "create":
			//Create the domain object and then throw it into cache
		case "purge":
			//Purge the domain object from cache
		}
	}
	return nil
}

// Watch for redis messages in the cache purge channel
// when one comes in, remove the record from the cache
func watchCacheChannel(rdc *redis.PubSub) {
	defer rdc.Close()
	log.Println("Watching for redis cache management messages...")
	ch := rdc.Channel()

	for msg := range ch {
		var cacheMsg CacheControlMessage
		json.Unmarshal([]byte(msg.Payload), &cacheMsg)
		// we can run this async without caring about returning a result
		// this is just "we have a record, give cacheMessageHandler() the msg
		// and move on with the next msg"
		go cacheMessageHandler(cacheMsg)
	}
}
