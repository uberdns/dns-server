package main

import (
	"log"
	"net"
	"time"

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
	case "CNAME":
		recordTypeConst = dns.TypeCNAME
	}

	msg.Question[0] = dns.Question{fqdn, recordTypeConst, dns.ClassINET}

	c := new(dns.Client)
	c.Timeout = time.Duration(2 * time.Second)
	in, _, err := c.Exchange(msg, "1.1.1.1:53")

	if err, ok := err.(net.Error); ok && err.Timeout() {
		debugMsg("Connection to upstream DNS timed out, retrying...")
		in, _, nerr := c.Exchange(msg, "1.1.1.1:53")
		if nerr != nil {
			debugMsg("Retry of upstream DNS timed out again. Fatal error.")
			log.Fatal(err)
		}
		return in.Answer
	} else if err != nil {
		log.Fatal(err)
	}

	return in.Answer
}
