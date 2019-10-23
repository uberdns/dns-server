package main

import (
	"fmt"
	"time"
)

func recordTTLWatcher(record Record, cachePurgeChan chan<- Record) {
	debugMsg("[TTL] Starting ttl watcher for cached record")

	ticker := time.NewTicker(time.Second)
       defer ticker.Stop()

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

func addRecordToCache(record Record, recSlice RecordMap, cacheChan chan<- Record, cachePurgeChan chan<- Record) error {
	// if record already exists in cache, do nothing
	recSlice.Lock()
	if recSlice.Records[record.ID] == record {
		return nil
	}
	recSlice.Unlock()
	debugMsg("[CACHE] Adding record to cache channel")
	record.DOB = time.Now()
	//recSlice[record.ID] = record
	cacheChan <- record
	debugMsg("[CACHE] Added record to cache channel")

	recordTTLWatcher(record, cachePurgeChan)

	return nil
}

func addDomainToCache(domain string, recSlice DomainMap, cacheChan chan<- Domain) Domain {
	recSlice.Lock()
	for i := range recSlice.Domains {
		if recSlice.Domains[i].Name == domain {
			recSlice.Unlock()
			return recSlice.Domains[i]
		}
	}
	recSlice.Unlock()
	domObj := Domain{
		ID:   int64(len(recSlice.Domains)),
		Name: domain,
	}
	debugMsg(fmt.Sprintf("[CACHE] Adding domain %s to cache channel", domObj.Name))
	cacheChan <- domObj
	debugMsg(fmt.Sprintf("[CACHE] Added domain %s to cache channel", domObj.Name))
	return domObj
}

func watchCache(cacheChan <-chan Record, cachePurgeChan <-chan Record, recSlice RecordMap) {
	for {
		select {
		case msg := <-cacheChan:
			debugMsg(fmt.Sprint("Record received over cache channel: ", msg))
			debugMsg("[CACHE] Adding record to slice")
			recSlice.Lock()
			recSlice.Records[msg.ID] = msg
			recSlice.Unlock()
			debugMsg("[CACHE] Added record to slice")
		case msg := <-cachePurgeChan:
			debugMsg("[CACHE] Removing record from slice")
			recSlice.Lock()
			delete(recSlice.Records, msg.ID)
			recSlice.Unlock()
			debugMsg("[CACHE] Removed record from slice")
		}
	}
}
