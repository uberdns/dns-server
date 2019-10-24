package main

import (
	"sync"
	"testing"
)

var benchRecords RecordMap
var recordChannel = make(chan Record)
var purgeChannel = make(chan Record)

func BenchmarkCreateCachedRecords(b *testing.B) {
	benchRecords.mu = new(sync.Mutex)
	benchRecords.Records = make(map[int]Record)
	for i := 0; i < b.N; i++ {
		var record = new(Record)
		record.TTL = 0
		go addRecordToCache(*record, benchRecords, recordChannel, purgeChannel)
		<-recordChannel
	}
}

func BenchmarkDeleteCachedRecords(b *testing.B) {
	benchRecords.mu = new(sync.Mutex)
	benchRecords.Records = make(map[int]Record)

	for i := 0; i < b.N; i++ {
		record := new(Record)
		record.TTL = 0
		benchRecords.Records = make(map[int]Record)
		benchRecords.Records[0] = *record
		benchRecords.DeleteRecord(*record)
	}
}
