package main

import (
	"fmt"
	"time"
)

func recordTTLWatcher(record Record, cachePurgeChan chan<- Record) {
	debugMsg("[TTL] Starting ttl watcher for cached record")

	ticker := time.NewTicker(time.Second)

	debugMsg("[TTL] Started ttl watcher for cached record")

	go func() {
		for range ticker.C {
			if (time.Now().Unix() - record.DOB.Unix()) > record.TTL {
				debugMsg("[TTL] Removing record via cache purge channel")
				cachePurgeChan <- record
				ticker.Stop()
				debugMsg("[TTL] Removed record via cache purge channel")
				return
			}
		}
	}()

}

func addRecordToCache(record Record, recSlice map[int]Record, cacheChan chan<- Record, cachePurgeChan chan<- Record) error {
	// if record already exists in cache, do nothing
	if recSlice[record.ID] == record {
		return nil
	}
	debugMsg("[CACHE] Adding record to cache channel")
	record.DOB = time.Now()
	//recSlice[record.ID] = record
	cacheChan <- record
	debugMsg("[CACHE] Added record to cache channel")

	recordTTLWatcher(record, cachePurgeChan)

	return nil
}

func watchCache(cacheChan <-chan Record, cachePurgeChan <-chan Record, recSlice map[int]Record) {
	for {
		select {
		case msg := <-cacheChan:
			debugMsg(fmt.Sprint("Record received over cache channel: ", msg))
			debugMsg("[CACHE] Adding record to slice")
			recSlice[msg.ID] = msg
			debugMsg("[CACHE] Added record to slice")
		case msg := <-cachePurgeChan:
			debugMsg("[CACHE] Removing record from slice")
			delete(recSlice, msg.ID)
			debugMsg("[CACHE] Removed record from slice")
		}
	}
}
