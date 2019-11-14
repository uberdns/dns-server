package main

import (
	"fmt"
	"time"

	"github.com/miekg/dns"

	log "github.com/sirupsen/logrus"
)

func recurseResolve(fqdn string, recordType uint16) []dns.RR {
	timeStart := time.Now()

	if string(fqdn[len(fqdn)-1]) != "." {
		fqdn = fqdn + "."
	}

	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, 1)

	msg.Question[0] = dns.Question{fqdn, recordType, dns.ClassINET}
	var answer []dns.RR

	var upstreamWinner string

	for i := range upstream_servers {
		c := new(dns.Client)
		c.Timeout = time.Duration(2 * time.Second)
		sockPath := fmt.Sprintf("%s:53", upstream_servers[i])
		in, _, err := c.Exchange(msg, sockPath)

		if err != nil {
			logger("recurse_dns").Error(err.Error())

			logger("recurse_dns").Infof("Retrying lookup due to failed upstream lookup on server %s", sockPath)
			continue
		}
		answer = in.Answer
		upstreamWinner = upstream_servers[i]
	}

	timeStop := time.Now()

	log.WithFields(log.Fields{
		"service":    "recurse_dns",
		"query_time": timeStop.Sub(timeStart).Seconds(),
		"resolver":   upstreamWinner,
		"query_type": dns.TypeToString[recordType],
	}).Info(fmt.Sprintf("Query: %s", fqdn))
	recordQueryCounter.WithLabelValues("recurse", dns.TypeToString[recordType]).Inc()
	return answer

	//	c := new(dns.Client)
	//	c.Timeout = time.Duration(2 * time.Second)
	//	in, _, err := c.Exchange(msg, "1.1.1.1:53")

	//	if err, ok := err.(net.Error); ok && err.Timeout() {
	//		log.Error(err.Error())
	//		log.Info("Retrying lookup due to upstream timeout")
	//		in, _, nerr := c.Exchange(msg, "1.1.1.1:53")
	//		if nerr != nil {
	//			log.Error("Retry of upstream DNS timed out again. Fatal error.")
	//			log.Error(err.Error())
	//			return []dns.RR{}
	//			//log.Fatal(err)
	//		}
	//		return in.Answer
	//	} else if err != nil {
	//		log.Error(err.Error())
	//	}

	//recordQueryCounter.WithLabelValues("recurse", recordType).Inc()

	//return in.Answer
}
