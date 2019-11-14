package main

import (
	"fmt"
	"time"
)

func recordTTLWatcher(record Record, cachePurgeChan chan<- Record) {

	logger("ttl_watcher").Debug("Starting ttl watcher for cached record")
	timer := time.NewTimer(time.Duration(record.TTL) * time.Second)
	defer timer.Stop()

	logger("ttl_watcher").Debug("Started ttl watcher for cached record")
	<-timer.C
	logger("ttl_watcher").Debug("Removing record via cache purge channel")
	cachePurgeChan <- record
	logger("ttl_watcher").Debug("Removed record via cache purge channel")
	return

}

func addRecordToCache(record Record, recSlice RecordMap, cacheChan chan<- Record, cachePurgeChan chan<- Record) error {
	// if record already exists in cache, do nothing
	if recSlice.Contains(record) {
		return nil
	}

	logger("cache").Debug("Adding record to cache channel")
	record.DOB = time.Now()
	//recSlice[record.ID] = record
	cacheChan <- record
	logger("cache").Debug("Added record to cache channel")

	go recordTTLWatcher(record, cachePurgeChan)

	return nil
}

func addDomainToCache(domain Domain, recSlice DomainMap, cacheChan chan<- Domain) error {
	if recSlice.Contains(domain) {
		return nil
	}

	logger("cache").Debug(fmt.Sprintf("Adding domain %s to cache channel", domain.Name))
	cacheChan <- domain
	logger("cache").Debug(fmt.Sprintf("Added domain %s to cache channel", domain.Name))
	return nil
}

func watchCache(cacheChan <-chan Record, cachePurgeChan <-chan Record, recSlice RecordMap) {
	for {
		select {
		case msg := <-cacheChan:
			logger("cache").Debug(fmt.Sprint("Record received over cache channel: ", msg))
			logger("cache").Debug("Adding record to slice")
			recSlice.AddRecord(msg)
			logger("cache").Debug("Added record to slice")
		case msg := <-cachePurgeChan:
			logger("cache").Debug(fmt.Sprint("Purge record signal received: ", msg))
			logger("cache").Debug("Removing record from slice")
			recSlice.DeleteRecord(msg)
			logger("cache").Debug("Removed record from slice")
		}
	}
}
