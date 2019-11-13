package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

func recordTTLWatcher(record Record, cachePurgeChan chan<- Record) {

	log.Info("[TTL] Starting ttl watcher for cached record")
	timer := time.NewTimer(time.Duration(record.TTL) * time.Second)
	defer timer.Stop()

	log.Info("[TTL] Started ttl watcher for cached record")
	<-timer.C
	log.Info("[TTL] Removing record via cache purge channel")
	cachePurgeChan <- record
	log.Info("[TTL] Removed record via cache purge channel")
	return

}

func addRecordToCache(record Record, recSlice RecordMap, cacheChan chan<- Record, cachePurgeChan chan<- Record) error {
	// if record already exists in cache, do nothing
	if recSlice.Contains(record) {
		return nil
	}

	log.Info("[CACHE] Adding record to cache channel")
	record.DOB = time.Now()
	//recSlice[record.ID] = record
	cacheChan <- record
	log.Info("[CACHE] Added record to cache channel")

	go recordTTLWatcher(record, cachePurgeChan)

	return nil
}

func addDomainToCache(domain Domain, recSlice DomainMap, cacheChan chan<- Domain) error {
	if recSlice.Contains(domain) {
		return nil
	}

	log.Info(fmt.Sprintf("[CACHE] Adding domain %s to cache channel", domain.Name))
	cacheChan <- domain
	log.Info(fmt.Sprintf("[CACHE] Added domain %s to cache channel", domain.Name))
	return nil
}

func watchCache(cacheChan <-chan Record, cachePurgeChan <-chan Record, recSlice RecordMap) {
	for {
		select {
		case msg := <-cacheChan:
			log.Info(fmt.Sprint("Record received over cache channel: ", msg))
			log.Info("[CACHE] Adding record to slice")
			recSlice.AddRecord(msg)
			log.Info("[CACHE] Added record to slice")
		case msg := <-cachePurgeChan:
			log.Info("[CACHE] Removing record from slice")
			recSlice.DeleteRecord(msg)
			log.Info("[CACHE] Removed record from slice")
		}
	}
}
