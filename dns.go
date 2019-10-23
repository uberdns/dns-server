package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type handler struct{}

func domainChannelHandler(channel <-chan Domain, domSlice map[int]Domain, domSliceMutex sync.RWMutex) {
	for {
		select {
		case msg := <-channel:
			debugMsg(fmt.Sprintf("[Domain] Adding domain %s to cache", msg.Name))
			domSliceMutex.Lock()
			domSlice[int(msg.ID)] = msg
			domSliceMutex.Unlock()
			debugMsg(fmt.Sprintf("[Domain] Added domain %s to cache", msg.Name))
		}
	}
}

func startListening(protocol string, port int) {
	srv := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: protocol}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to set tcp listener %s\n", err.Error())
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
	for _, d := range domains {
		if topLevelDomain == d.Name {
			realDomain = d
		}
	}

	if (Domain{}) == realDomain {
		debugMsg("Starting recursive lookup")

		var recurseDomain Domain
		recursiveDomainMutex.Lock()
		for _, d := range recursiveDomains {
			if topLevelDomain == d.Name {
				recurseDomain = d
			}
		}
		recursiveDomainMutex.Unlock()

		if (Domain{}) == recurseDomain {
			debugMsg("Recurse domain not found, performing lookup")
			rr := recurseResolve(domain, "A")
			for i := range rr {
				debugMsg("Adding recursive domain to local cache")
				rrd := addDomainToCache(topLevelDomain, recursiveDomains, recursiveDomainMutex, recursiveDomainChannel)
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

					recursiveRecordMutex.Lock()
					rrr.ID = len(recursiveRecords)
					recursiveRecordMutex.Unlock()

					go addRecordToCache(rrr, recursiveRecords, recursiveRecordMutex, recursiveCacheChannel, recursiveCachePurgeChannel)
				}
			}
			w.WriteMsg(&msg)
		} else {
			debugMsg("Recurse domain found in cache")
			var device Record
			recursiveRecordMutex.Lock()
			for i, j := range recursiveRecords {
				if j.DomainID != recurseDomain.ID {
					continue
				}
				if recursiveRecords[i].Name == subdomain {
					device = recursiveRecords[i]
				}
			}
			recursiveRecordMutex.Unlock()

			if (Record{}) == device {
				rr := recurseResolve(domain, "A")

				debugMsg("Recurse record not found in cache, performing lookup")

				for i := range rr {
					msg.Answer = append(msg.Answer, rr[i])
					switch rr[i].Header().Rrtype {
					case dns.TypeA:
						rrr := Record{
							ID:       len(recursiveRecords),
							Name:     subdomain,
							TTL:      int64(rr[i].(*dns.A).Hdr.Ttl),
							IP:       rr[i].(*dns.A).A.String(),
							Created:  time.Now(),
							DOB:      time.Now(),
							DomainID: recurseDomain.ID,
						}
						go addRecordToCache(rrr, recursiveRecords, recursiveRecordMutex, recursiveCacheChannel, recursiveCachePurgeChannel)
					}
				}
				w.WriteMsg(&msg)

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
		}
	} else {
		// Domain matches, we should continue to search
		var device Record
		recordMutex.Lock()
		for i, j := range records {
			if j.DomainID != realDomain.ID {
				continue
			}
			if records[i].Name == subdomain {
				device = records[i]
			}
		}
		recordMutex.Unlock()

		if (Record{}) == device {
			//No existing records found in local cache, perform sql lookup
			// if the sql lookup fails then we give up
			device, _ = getRecordFromHost(subdomain, realDomain.ID)

			// Ensure non-empty device
			if (Record{}) != device {
				log.Println("Non-cached record, adding to cache")
				device.DOB = time.Now()
				go addRecordToCache(device, records, recordMutex, recordCacheChannel, recordCachePurgeChannel)
			}

		}

		switch r.Question[0].Qtype {
		case dns.TypeA:
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(device.TTL)},
				A:   net.ParseIP(device.IP),
			})
		}
	}
	w.WriteMsg(&msg)
}
