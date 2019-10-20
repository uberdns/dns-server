package main

import (
	"fmt"
	"time"
)

func addRecordToCache(record Record) error {
	// if record already exists in cache, do nothing
	for i := range records {
		if records[i].ID == record.ID {
			return nil
		}
	}
	// Set DOB to now as we're creating the object in cache
	record.DOB = time.Now()
	records[record.ID] = record
	return nil
}

func removeRecordFromCache(record Record) error {
	delete(records, record.ID)
	return nil
}

// Run through record objects currently cached and evaluate
// whether we need to expire them (remove)
func cleanCache() error {
	for i := range records {
		if (time.Now().Unix() - records[i].DOB.Unix()) > records[i].TTL {
			debugMsg(fmt.Sprintf("Expiring uberdns record %s", records[i].Name))
			delete(records, i)
		}
	}

	for k := range recursiveRecords {
		if (time.Now().Unix() - recursiveRecords[k].DOB.Unix()) > recursiveRecords[k].TTL {
			debugMsg(fmt.Sprintf("Expiring recurse record %s", recursiveRecords[k].Name))
			delete(recursiveRecords, k)
		}
	}
	return nil
}
