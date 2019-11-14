package main

import (
	"database/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
)

func populateData(done chan<- bool) {
	log.Info("[DATA] Populating data.")
	query := "SELECT id, name FROM dns_domain"
	log.Debug("Query: " + query)
	dq, err := dbConn.Prepare(query)

	if err != nil {
		log.Error(err)
		return
	}

	defer dq.Close()

	rows, err := dq.Query()
	if err != nil {
		log.Error(err)
		return
	}

	for rows.Next() {
		var (
			id   int64
			name string
		)

		if err := rows.Scan(&id, &name); err != nil {
			log.Error(err)
		}
		log.Debug("Domain found: " + name)
		domains.Domains[int(id)] = Domain{ID: id, Name: name}
	}
	log.Info("[DATA] Data populated.")

	done <- true
}

func getDomain(domainName string) (Domain, error) {
	var domain Domain

	query := "SELECT id FROM dns_domain WHERE name = ?"
	dq, err := dbConn.Prepare(query)
	log.Debug("Query: " + query)

	if err != nil {
		log.Error(err)
	}

	defer dq.Close()

	err = dq.QueryRow(domainName).Scan(&domain.ID)
	if err != nil {
		log.Error(err)
	}
	domain.Name = domainName
	log.Debug(fmt.Sprintf("Found domain ID %d", domain.ID))

	return domain, nil
}

func getRecordFromHost(host string, domainID int64) (Record, error) {
	var record Record

	query := "SELECT id, name, ip_address, ttl, domain_id FROM dns_record WHERE name = ? AND domain_id = ?"
	dq, err := dbConn.Prepare(query)
	log.Debug("Query: " + query)

	if err != nil {
		log.Error(err)
	}

	defer dq.Close()

	err = dq.QueryRow(host, domainID).Scan(&record.ID, &record.Name, &record.IP, &record.TTL, &record.DomainID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Warning("Lookup failed but domain was valid.")
		} else {
			log.Error(err)
		}
	}

	return record, nil

}
