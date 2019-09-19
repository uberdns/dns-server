package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/miekg/dns"
)

type Record struct {
	ID   int
	Name string
	IP   string
	TTL  int64     //TTL for caching
	DOB  time.Time //time record created, used for cache expiry
}

var records = []Record{}

var dbConn sql.DB

func dbConnect() error {
	conn := "root:root@tcp(127.0.0.1:3306)/ddns"
	dbc, err := sql.Open("mysql", conn)

	if err != nil {
		return err
	}

	//defer dbc.Close()

	err = dbc.Ping()
	if err != nil {
		panic(err.Error())
	}

	dbConn = *dbc
	return nil
}

func getRecordFromHost(host string) (Record, error) {
	var record Record

	query := "SELECT id, name, ip, ttl FROM hosts WHERE name = ?"

	dq, err := dbConn.Prepare(query)

	if err != nil {
		log.Fatal(err)
	}

	defer dq.Close()

	err = dq.QueryRow(host).Scan(&record.ID, &record.Name, &record.IP, &record.TTL)
	if err != nil {
		panic(err.Error())
	}

	return record, nil

}

type handler struct{}

func (this *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	domain := msg.Question[0].Name
	msg.Authoritative = true

	// To-Do:
	// - On successful lookup, add to records list (done)
	// - Check known records list before performing mysql query (done)
	// - If record in known record list, check for cache expired (done)

	match, err := regexp.MatchString(".*google.com.", domain)
	if err != nil {
		log.Println(err)
	}

	if !match {
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(""),
		})
		//w.WriteMsg(&msg)
	} else {
		var device Record
		for i := range records {
			if records[i].Name == strings.Split(domain, ".")[0] {
				device = records[i]
				fmt.Println("Record found in cache")
			}
		}

		if (Record{}) == device {
			device, err = getRecordFromHost(strings.Split(domain, ".")[0])
			if err != nil {
				panic(err.Error())
			}
			// Ensure non-empty device
			if (Record{}) != device {
				fmt.Println("Non-cached record, adding to cache")
				device.DOB = time.Now()
				records = append(records, device)
			}

		}

		fmt.Println("TTL: ", device.TTL)
		fmt.Println("DOB: ", device.DOB)
		fmt.Println("Time in cache: ", (time.Since(device.DOB)))

		switch r.Question[0].Qtype {
		case dns.TypeA:
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(device.IP),
			})
		}
	}
	w.WriteMsg(&msg)
}

func cleanCache() error {
	for i := range records {
		if (time.Now().Unix() - records[i].DOB.Unix()) > records[i].TTL {
			log.Println("Cleaning up cached record ", records[i])
			// Delete record from struct
			records = append(records[:i], records[i+1:]...)
		}
	}
	return nil
}

func main() {
	go func() {
		err := dbConnect()

		if err != nil {
			log.Fatal(err)
		}
	}()

	// Clean up records that exceed their TTL
	go func() {
		for {
			if err := cleanCache(); err != nil {
				log.Fatalf("Unable to clean up cache %s\n", err.Error())
			}
		}
	}()

	go func() {
		srv := &dns.Server{Addr: ":" + strconv.Itoa(53), Net: "tcp"}
		srv.Handler = &handler{}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set tcp listener %s\n", err.Error())
		}
	}()

	go func() {
		srv := &dns.Server{Addr: ":" + strconv.Itoa(53), Net: "udp"}
		srv.Handler = &handler{}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping\n", s)

}
