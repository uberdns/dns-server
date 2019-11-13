package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func debugMsg(msg string) {
	if DEBUG {
		log.Info("[DEBUG] " + msg)
	}
}

func debugDomainHandler(w http.ResponseWriter, r *http.Request) {
	type Debug struct {
		RecursiveCount   int
		RecursiveDomains map[int]Domain
	}

	var data = Debug{
		RecursiveCount:   len(recursiveDomains.Domains),
		RecursiveDomains: recursiveDomains.Domains,
	}

	jd, _ := json.Marshal(data)
	w.Write([]byte(jd))
}

func debugRecordHandler(w http.ResponseWriter, r *http.Request) {
	type Debug struct {
		RecursiveCount   int
		RecursiveRecords map[int]Record
	}
	var data = Debug{
		RecursiveCount:   len(recursiveRecords.Records),
		RecursiveRecords: recursiveRecords.Records,
	}

	jd, _ := json.Marshal(data)
	w.Write([]byte(jd))
}
