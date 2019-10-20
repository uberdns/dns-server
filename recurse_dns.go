package main

import (
	"log"

	"github.com/miekg/dns"
)

func recurseResolve(fqdn string) *dns.A {
	if string(fqdn[len(fqdn)-1]) != "." {
		fqdn = fqdn + "."
	}

	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, 1)
	msg.Question[0] = dns.Question{fqdn, dns.TypeA, dns.ClassINET}

	c := new(dns.Client)
	in, _, err := c.Exchange(msg, "1.1.1.1:53")

	if err != nil {
		log.Fatal(err)
	}

	if t, ok := in.Answer[0].(*dns.A); ok {
		return t
	}
	return nil
}
