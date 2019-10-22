package main

import (
	"fmt"
	"sync"
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

func addRecordToCache(record Record, recSlice map[int]Record, recSliceMutex sync.RWMutex, cacheChan chan<- Record, cachePurgeChan chan<- Record) error {
	// if record already exists in cache, do nothing
	recSliceMutex.Lock()
	if recSlice[record.ID] == record {
		return nil
	}
	recSliceMutex.Unlock()
	debugMsg("[CACHE] Adding record to cache channel")
	record.DOB = time.Now()
	//recSlice[record.ID] = record
	cacheChan <- record
	debugMsg("[CACHE] Added record to cache channel")

	recordTTLWatcher(record, cachePurgeChan)

	return nil
}

func addDomainToCache(domain string, recSlice map[int]Domain, recSliceMutex sync.RWMutex, cacheChan chan<- Domain) Domain {
	recSliceMutex.Lock()
	for i := range recSlice {
		if recSlice[i].Name == domain {
			recSliceMutex.Unlock()
			return recSlice[i]
		}
	}
	recSliceMutex.Unlock()
	domObj := Domain{
		ID:   int64(len(recSlice)),
		Name: domain,
	}
	debugMsg(fmt.Sprintf("[CACHE] Adding domain %s to cache channel", domObj.Name))
	cacheChan <- domObj
	debugMsg(fmt.Sprintf("[CACHE] Added domain %s to cache channel", domObj.Name))
	return domObj
}

func watchCache(cacheChan <-chan Record, cachePurgeChan <-chan Record, recSlice map[int]Record, recSliceMutex sync.RWMutex) {
	for {
		select {
		case msg := <-cacheChan:
			debugMsg(fmt.Sprint("Record received over cache channel: ", msg))
			debugMsg("[CACHE] Adding record to slice")
			recSliceMutex.Lock()
			recSlice[msg.ID] = msg
			recSliceMutex.Unlock()
			debugMsg("[CACHE] Added record to slice")
		case msg := <-cachePurgeChan:
			debugMsg("[CACHE] Removing record from slice")
			recSliceMutex.Lock()
			delete(recSlice, msg.ID)
			recSliceMutex.Unlock()
			debugMsg("[CACHE] Removed record from slice")
		}
	}
}
