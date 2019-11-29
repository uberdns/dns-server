package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var commonLogger = log.WithFields(log.Fields{
	"service": "dns",
})

func (d *DomainMap) GetDomains() map[int]Domain {
	var domains = make(map[int]Domain)
	d.mu.Lock()
	domains = d.Domains
	d.mu.Unlock()

	return domains
}

func (d *DomainMap) GetDomainByID(id int) Domain {
	var domain Domain
	d.mu.Lock()
	domain = d.Domains[id]
	d.mu.Unlock()

	return domain
}

func (d *DomainMap) GetDomainByName(name string) Domain {
	d.mu.Lock()
	for i := range d.Domains {
		if d.Domains[i].Name == name {
			d.mu.Unlock()
			return d.Domains[i]
		}
	}
	d.mu.Unlock()
	return Domain{}
}

func (d *DomainMap) AddDomain(domain Domain) {
	d.mu.Lock()
	d.Domains[int(domain.ID)] = domain
	d.mu.Unlock()
}

func (d *DomainMap) DeleteDomain(domain Domain) {
	d.mu.Lock()
	delete(d.Domains, int(domain.ID))
	d.mu.Unlock()
}

func (d *DomainMap) Contains(domain Domain) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i := range d.Domains {
		// return if id match
		if d.Domains[i].ID == domain.ID {
			return true
		}
		// if id doesnt match, this is a problem but we should still
		// return that the map contains the domain
		if d.Domains[i].Name == domain.Name {
			return true
		}
	}

	return false
}

func (d *DomainMap) Count() int {
	var i int
	d.mu.Lock()
	i = len(d.Domains)
	d.mu.Unlock()
	return i
}

func (r *RecordMap) GetRecords() map[int]Record {
	var records = make(map[int]Record)
	r.mu.Lock()
	records = r.Records
	r.mu.Unlock()

	return records
}

func (r *RecordMap) GetRecordByID(id int) Record {
	var record Record
	r.mu.Lock()
	record = r.Records[id]
	r.mu.Unlock()

	return record
}

func (r *RecordMap) GetRecordByName(name string, domainId int64) Record {
	var record Record
	r.mu.Lock()
	for i := range r.Records {
		if r.Records[i].DomainID != domainId {
			continue
		}

		if r.Records[i].Name == name {
			record = r.Records[i]
		}
	}
	r.mu.Unlock()

	return record
}

func (r *RecordMap) Contains(record Record) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Records {
		if r.Records[i].Name == record.Name {
			return true
		}
	}

	return false
}

func (r *RecordMap) AddRecord(record Record) {
	r.mu.Lock()
	r.Records[record.ID] = record
	r.mu.Unlock()
}

func (r *RecordMap) DeleteRecord(record Record) {
	r.mu.Lock()
	delete(r.Records, record.ID)
	r.mu.Unlock()
}

func (r *RecordMap) Count() int {
	var i int
	r.mu.Lock()
	i = len(r.Records)
	r.mu.Unlock()
	return i
}

type handler struct{}

func domainChannelHandler(channel <-chan Domain, domSlice DomainMap) {
	for {
		select {
		case msg := <-channel:
			logger("cache").Debug(fmt.Sprintf("Adding domain %s to cache", msg.Name))
			domSlice.AddDomain(msg)
			logger("cache").Debug(fmt.Sprintf("Added domain %s to cache", msg.Name))
		}
	}
}

func startListening(protocol string, port int) {
	srv := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: protocol}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		logger("dns").Fatalf("Failed to set %s listener %s\n", protocol, err.Error())
	}
}

func (fuck *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	timeStart := time.Now()
	msg := dns.Msg{}
	msg.SetReply(r)
	domain := msg.Question[0].Name
	msg.Authoritative = true

	cleanDomain := strings.TrimRight(domain, ".")
	domainSplit := strings.Split(cleanDomain, ".")
	// Capture domain name plus TLD
	var subdomain string
	var topLevelDomain string
	if strings.Count(cleanDomain, ".") > 1 {
		subdomain = domainSplit[0]
		domainSplit := strings.Split(cleanDomain, ".")
		var s = []string{domainSplit[len(domainSplit)-2], domainSplit[len(domainSplit)-1]}
		topLevelDomain = strings.Join(s, ".")
	} else {
		topLevelDomain = cleanDomain
	}

	logger("dns").Debug(fmt.Sprintf("Query received: %s", msg.Question[0].Name))

	// Currently we only cache A records, if the question is of a non-A type - default to recursiveResolve lookup
	if r.Question[0].Qtype != dns.TypeA {
		rr := recurseResolve(msg.Question[0].Name, r.Question[0].Qtype)
		for i := range rr {
			msg.Answer = append(msg.Answer, rr[i])
		}
	}

	var realDomain Domain
	copyDomains := domains.GetDomains()
	for _, d := range copyDomains {
		if topLevelDomain == d.Name {
			realDomain = d
		}
	}

	if (Domain{}) == realDomain {
		logger("recurse_dns").Debug(fmt.Sprintf("Starting recursive lookup: %s", msg.Question[0].Name))

		var recurseDomain Domain
		realDomain.Name = topLevelDomain
		if recursiveDomains.Contains(realDomain) {
			recurseDomain = recursiveDomains.GetDomainByName(topLevelDomain)
		}

		if (Domain{}) == recurseDomain {
			logger("recurse_dns").Debug("Recurse domain not found, performing lookup")
			domObj := Domain{
				ID:   int64(recursiveDomains.Count()),
				Name: topLevelDomain,
			}
			rr := recurseResolve(domain, r.Question[0].Qtype)
			for i := range rr {
				logger("recurse_dns").Debug("Adding recurive domain to local cache")
				addDomainToCache(domObj, recursiveDomains, recursiveDomainChannel)
				msg.Answer = append(msg.Answer, rr[i])
				// if its an A record, we should cache it!
				switch rr[i].Header().Rrtype {
				case dns.TypeA:
					rrr := Record{
						Name:     subdomain,
						IP:       rr[i].(*dns.A).A.String(),
						TTL:      int64(rr[i].(*dns.A).Hdr.Ttl),
						Created:  time.Now(),
						DOB:      time.Now(),
						DomainID: domObj.ID,
					}

					rrr.ID = recursiveRecords.Count()

					recursiveRecords.mu.Lock()
					copyr := recursiveRecords
					recursiveRecords.mu.Unlock()

					go addRecordToCache(rrr, copyr, recursiveCacheChannel, recursiveCachePurgeChannel)
				}
			}
			w.WriteMsg(&msg)
		} else {
			logger("recurse_dns").Debug("Recurse domain found in cache")
			var device = recursiveRecords.GetRecordByName(subdomain, recurseDomain.ID)

			if (Record{}) == device {
				rr := recurseResolve(domain, r.Question[0].Qtype)

				logger("recurse_dns").Debug("Recurse record not found in cache, performing lookup")

				for i := range rr {
					msg.Answer = append(msg.Answer, rr[i])
					switch rr[i].Header().Rrtype {
					case dns.TypeA:
						rrr := Record{
							ID:       recursiveRecords.Count(),
							Name:     subdomain,
							TTL:      int64(rr[i].(*dns.A).Hdr.Ttl),
							IP:       rr[i].(*dns.A).A.String(),
							Created:  time.Now(),
							DOB:      time.Now(),
							DomainID: recurseDomain.ID,
						}
						go addRecordToCache(rrr, recursiveRecords, recursiveCacheChannel, recursiveCachePurgeChannel)
					}
				}
				w.WriteMsg(&msg)
				recordQueryCounter.WithLabelValues("recurse", dns.TypeToString[r.Question[0].Qtype]).Inc()

			} else {
				logger("recurse_dns").Debug("Returning cached recursive record")

				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(device.TTL)},
					A:   net.ParseIP(device.IP),
				})
			}
			err := w.WriteMsg(&msg)
			if err != nil {
				log.Fatal(err)
			}
			recordQueryCounter.WithLabelValues("recurse", dns.TypeToString[r.Question[0].Qtype]).Inc()
		}
	} else {

		// Domain matches, we should continue to search
		var device Record
		copyRecords := records.GetRecords()
		for i, j := range copyRecords {
			if j.DomainID != realDomain.ID {
				continue
			}
			if copyRecords[i].Name == subdomain {
				device = copyRecords[i]
			}
		}

		if (Record{}) == device {
			//No existing records found in local cache, perform sql lookup
			// if the sql lookup fails then we give up
			device, _ = getRecordFromHost(subdomain, realDomain.ID)

			// Ensure non-empty device
			if (Record{}) != device {
				logger("dns").Debug(fmt.Sprintf("Adding record %s.%s to cache", subdomain, realDomain.Name))
				device.DOB = time.Now()
				go addRecordToCache(device, records, recordCacheChannel, recordCachePurgeChannel)
			}

		}

		switch r.Question[0].Qtype {
		case dns.TypeA:
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(device.TTL)},
				A:   net.ParseIP(device.IP),
			})
		}
		recordQueryCounter.WithLabelValues("uberdns", dns.TypeToString[r.Question[0].Qtype]).Inc()
	}
	// Cached dns record
	w.WriteMsg(&msg)

	timeStop := time.Now()
	log.WithFields(log.Fields{
		"service":     "dns",
		"query_time":  timeStop.Sub(timeStart).Seconds(),
		"query_type":  dns.TypeToString[r.Question[0].Qtype],
		"local_addr":  w.LocalAddr().String(),
		"remote_addr": w.RemoteAddr().String(),
	}).Info(fmt.Sprintf("Query: %s", domain))
}
