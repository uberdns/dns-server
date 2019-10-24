package main

import (
	"fmt"
	"time"
)

func recordTTLWatcher(record Record, cachePurgeChan chan<- Record) {

	debugMsg("[TTL] Starting ttl watcher for cached record")
	timer := time.NewTimer(time.Duration(record.TTL) * time.Second)
	defer timer.Stop()

	debugMsg("[TTL] Started ttl watcher for cached record")
	<-timer.C
	debugMsg("[TTL] Removing record via cache purge channel")
	cachePurgeChan <- record
	debugMsg("[TTL] Removed record via cache purge channel")
	return

}

func addRecordToCache(record Record, recSlice RecordMap, cacheChan chan<- Record, cachePurgeChan chan<- Record) error {
	// if record already exists in cache, do nothing
	if recSlice.Contains(record) {
		return nil
	}

	debugMsg("[CACHE] Adding record to cache channel")
	record.DOB = time.Now()
	//recSlice[record.ID] = record
	cacheChan <- record
	debugMsg("[CACHE] Added record to cache channel")

	go recordTTLWatcher(record, cachePurgeChan)

	return nil
}

func addDomainToCache(domain Domain, recSlice DomainMap, cacheChan chan<- Domain) error {
	if recSlice.Contains(domain) {
		return nil
	}

	debugMsg(fmt.Sprintf("[CACHE] Adding domain %s to cache channel", domain.Name))
	cacheChan <- domain
	debugMsg(fmt.Sprintf("[CACHE] Added domain %s to cache channel", domain.Name))
	return nil
}

func watchCache(cacheChan <-chan Record, cachePurgeChan <-chan Record, recSlice RecordMap) {
	for {
		select {
		case msg := <-cacheChan:
			debugMsg(fmt.Sprint("Record received over cache channel: ", msg))
			debugMsg("[CACHE] Adding record to slice")
			recSlice.AddRecord(msg)
			debugMsg("[CACHE] Added record to slice")
		case msg := <-cachePurgeChan:
			debugMsg("[CACHE] Removing record from slice")
			recSlice.DeleteRecord(msg)
			debugMsg("[CACHE] Removed record from slice")
		}
	}
}
