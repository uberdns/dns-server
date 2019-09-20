package main

import (
	"os"
	"testing"
	"time"
)

var testDomains []Domain
var testRecords []Record

func TestPopulateData(*testing.T) {
	testDomain := Domain{
		ID:   1,
		Name: "test.com",
	}
	testDomains = append(testDomains, testDomain)

	testRecord := Record{
		ID:       1,
		Name:     "test",
		IP:       "127.0.0.1",
		TTL:      30,
		DOB:      time.Now(),
		DomainID: 1,
	}

	testRecords = append(testRecords, testRecord)

	if len(domains) != 0 && len(records) != 0 {
		return
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
