package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func (d *DomainMap) GetDomains() map[int]Domain {
	var domains = make(map[int]Domain)
	d.mu.Lock()
	domains = d.Domains
	d.mu.Unlock()

	return domains
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

func (d *DomainMap) Sum() int {
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

func (r *RecordMap) Sum() int {
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
			debugMsg(fmt.Sprintf("[Domain] Adding domain %s to cache", msg.Name))
			domSlice.AddDomain(msg)
			debugMsg(fmt.Sprintf("[Domain] Added domain %s to cache", msg.Name))
		}
	}
}

func startListening(protocol string, port int) {
	srv := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: protocol}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to set %s listener %s\n", protocol, err.Error())
	}
}

func (fuck *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
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

	switch r.Question[0].Qtype {
	case dns.TypeTXT:
		rr := recurseResolve(msg.Question[0].Name, "TXT")
		for i := range rr {
			msg.Answer = append(msg.Answer, rr[i])
		}
		// Always send response back to client and then do whatever else
		w.WriteMsg(&msg)
		return
	case dns.TypeMX:
		rr := recurseResolve(msg.Question[0].Name, "MX")
		for i := range rr {
			msg.Answer = append(msg.Answer, rr[i])
		}
		w.WriteMsg(&msg)
		return
	case dns.TypeAAAA:
		rr := recurseResolve(msg.Question[0].Name, "AAAA")
		for i := range rr {
			msg.Answer = append(msg.Answer, rr[i])
		}
		w.WriteMsg(&msg)
		return
	case dns.TypeCNAME:
		rr := recurseResolve(msg.Question[0].Name, "CNAME")
		for i := range rr {
			msg.Answer = append(msg.Answer, rr[i])
		}
		w.WriteMsg(&msg)
		return
	}

	var realDomain Domain
	copyDomains := domains.GetDomains()
	for _, d := range copyDomains {
		if topLevelDomain == d.Name {
			realDomain = d
		}
	}

	if (Domain{}) == realDomain {
		debugMsg("Starting recursive lookup")

		var recurseDomain Domain
		copyDomains := recursiveDomains.GetDomains()
		for _, d := range copyDomains {
			if topLevelDomain == d.Name {
				recurseDomain = d
			}
		}

		if (Domain{}) == recurseDomain {
			debugMsg("Recurse domain not found, performing lookup")
			rr := recurseResolve(domain, "A")
			for i := range rr {
				debugMsg("Adding recurive domain to local cache")
				rrd := addDomainToCache(topLevelDomain, recursiveDomains, recursiveDomainChannel)
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
						DomainID: rrd.ID,
					}

					rrr.ID = len(recursiveRecords.GetRecords())

					recursiveRecords.mu.Lock()
					copyr := recursiveRecords
					recursiveRecords.mu.Unlock()

					go addRecordToCache(rrr, copyr, recursiveCacheChannel, recursiveCachePurgeChannel)
				}
			}
			w.WriteMsg(&msg)
		} else {
			debugMsg("Recurse domain found in cache")
			var device Record
			copyRecords := recursiveRecords.GetRecords()
			for i, j := range copyRecords {
				if j.DomainID != recurseDomain.ID {
					continue
				}
				if copyRecords[i].Name == subdomain {
					device = copyRecords[i]
				}
			}

			if (Record{}) == device {
				rr := recurseResolve(domain, "A")

				debugMsg("Recurse record not found in cache, performing lookup")

				for i := range rr {
					msg.Answer = append(msg.Answer, rr[i])
					switch rr[i].Header().Rrtype {
					case dns.TypeA:
						rrr := Record{
							ID:       len(recursiveRecords.Records),
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
				recordQueryCounter.WithLabelValues("recurse", "A").Inc()

			} else {
				debugMsg("Returning cached recursive record")

				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(device.TTL)},
					A:   net.ParseIP(device.IP),
				})
			}
			err := w.WriteMsg(&msg)
			if err != nil {
				log.Fatal(err)
			}
			recordQueryCounter.WithLabelValues("recurse", "A").Inc()
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
				log.Println("Non-cached record, adding to cache")
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
		recordQueryCounter.WithLabelValues("uberdns", "A").Inc()
	}
	w.WriteMsg(&msg)
}
