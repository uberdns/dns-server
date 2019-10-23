package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"

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
			addRecordToCache(record, records, recordCacheChannel, recordCachePurgeChannel)
		case "purge":
			recordCachePurgeChannel <- record
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
func watchCacheChannel(rdc *redis.Client, cacheChannel string) {
	log.Printf("[REDIS] Subscribing to %s", cacheChannel)
	pubsub := rdc.Subscribe(cacheChannel)
	defer pubsub.Close()
	log.Println("[REDIS] Watching for cache management messages...")
	ch := pubsub.Channel()

	for msg := range ch {
		var cacheMsg CacheControlMessage
		json.Unmarshal([]byte(msg.Payload), &cacheMsg)
		// we can run this async without caring about returning a result
		// this is just "we have a record, give cacheMessageHandler() the msg
		// and move on with the next msg"
		go cacheMessageHandler(cacheMsg)
	}
}

func redisConnect(redisHost string, redisPassword string, redisDB int) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Ping/Pong - (Will be) Used for health check
	go func() {
		for {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for range ticker.C {
			    _, err := redisClient.Ping().Result()
			    if err != nil {
			        log.Println("[REDIS] Unable to communicate with " + redisHost)
			    }
			}

		}
	}()

	return redisClient
}
