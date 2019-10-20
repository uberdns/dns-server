package main

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type handler struct{}

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
		topLevelDomain = strings.Join(domainSplit[1:], ".")
	} else {
		topLevelDomain = cleanDomain
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
		for _, d := range recursiveDomains {
			if domain == d.Name {
				recurseDomain = d
			}
		}

		if (Domain{}) == recurseDomain {
			debugMsg("Recurse domain not found, performing lookup")
			rr := recurseResolve(domain)
			if rr != nil {
				rrd := Domain{
					ID:   int64(len(recursiveDomains)),
					Name: rr.Hdr.Name,
				}
				debugMsg("Adding recursive domain to local cache")
				recursiveDomains[int(rrd.ID)] = rrd
				msg.Answer = append(msg.Answer, rr)
				w.WriteMsg(&msg)
				rrr := Record{
					ID:       len(recursiveRecords),
					Name:     subdomain,
					IP:       rr.A.String(),
					TTL:      int64(rr.Hdr.Ttl),
					Created:  time.Now(),
					DOB:      time.Now(),
					DomainID: rrd.ID,
				}
				recursiveRecords[rrr.ID] = rrr
			} else {
				// if we dont know this domain, bail and return an empty set
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP(""),
				})
				w.WriteMsg(&msg)
			}
		} else {
			debugMsg("Recurse domain found in cache")
			var device Record
			for i, j := range recursiveRecords {
				if j.DomainID != recurseDomain.ID {
					continue
				}
				if recursiveRecords[i].Name == subdomain {
					device = recursiveRecords[i]
				}
			}

			if (Record{}) == device {
				rr := recurseResolve(domain)

				debugMsg("Recurse record not found in cache, performing lookup")

				msg.Answer = append(msg.Answer, rr)
				w.WriteMsg(&msg)

				rrr := Record{
					ID:       len(recursiveRecords),
					Name:     subdomain,
					TTL:      int64(rr.Hdr.Ttl),
					IP:       rr.A.String(),
					Created:  time.Now(),
					DOB:      time.Now(),
					DomainID: recurseDomain.ID,
				}
				recursiveRecords[rrr.ID] = rrr
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
		for i, j := range records {
			if j.DomainID != realDomain.ID {
				continue
			}
			if records[i].Name == subdomain {
				device = records[i]
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
				records[device.ID] = device
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