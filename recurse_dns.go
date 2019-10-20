package main

import (
	"log"

	"github.com/miekg/dns"
)

func recurseResolve(fqdn string, recordType string) []dns.RR {
	if string(fqdn[len(fqdn)-1]) != "." {
		fqdn = fqdn + "."
	}

	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, 1)

	var recordTypeConst uint16

	switch recordType {
	case "A":
		recordTypeConst = dns.TypeA
	case "MX":
		recordTypeConst = dns.TypeMX
	case "TXT":
		recordTypeConst = dns.TypeTXT
	case "AAAA":
		recordTypeConst = dns.TypeAAAA
	}

	msg.Question[0] = dns.Question{fqdn, recordTypeConst, dns.ClassINET}

	c := new(dns.Client)
	in, _, err := c.Exchange(msg, "1.1.1.1:53")

	if err != nil {
		log.Fatal(err)
	}

	return in.Answer
}
