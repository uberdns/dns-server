// To-Do:
// - prometheus exporter
// - promtool
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"net/http/pprof"

	_ "github.com/go-sql-driver/mysql"
	"github.com/miekg/dns"
	"gopkg.in/ini.v1"
)

type Domain struct {
	ID   int64
	Name string
}
type Record struct {
	ID       int
	Name     string
	IP       string
	TTL      int64     //TTL for caching
	DOB      time.Time //time record created, used for cache expiry
	DomainID int64
}

var domains = []Domain{}
var records = []Record{}

var dbConn sql.DB

func dbConnect(username string, password string, host string, port int, database string) error {
	conn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, database)
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

func populateData() error {
	query := "SELECT id, name FROM dns_domain"
	dq, err := dbConn.Prepare(query)

	if err != nil {
		return err
	}

	defer dq.Close()

	rows, err := dq.Query()
	if err != nil {
		return err
	}

	for rows.Next() {
		var (
			id   int64
			name string
		)

		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		fmt.Println("Found domain: ", name)

		domains = append(domains, Domain{ID: id, Name: name})
	}

	return nil
}

func getDomain(domainName string) (Domain, error) {
	var domain Domain

	query := "SELECT id FROM dns_domain WHERE name = ?"

	dq, err := dbConn.Prepare(query)

	if err != nil {
		panic(err.Error())
	}

	defer dq.Close()

	err = dq.QueryRow(domainName).Scan(&domain.ID)
	if err != nil {
		panic(err.Error())
	}
	domain.Name = domainName

	return domain, nil
}

func getRecordFromHost(host string, domain_id int64) (Record, error) {
	var record Record

	query := "SELECT id, name, ip_address, ttl, domain_id FROM dns_record WHERE name = ? AND domain_id = ?"
	fmt.Println(query)
	fmt.Println("name: ", host)
	fmt.Println("DID: ", domain_id)
	dq, err := dbConn.Prepare(query)

	if err != nil {
		log.Fatal(err)
	}

	defer dq.Close()

	err = dq.QueryRow(host, domain_id).Scan(&record.ID, &record.Name, &record.IP, &record.TTL, &record.DomainID)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("Lookup failed but domain was valid.")
		} else {
			log.Fatal(err)
		}
	}

	return record, nil

}

type handler struct{}

func (this *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	domain := msg.Question[0].Name
	msg.Authoritative = true

	top_level := strings.TrimRight(strings.Join(strings.Split(domain, ".")[1:], "."), ".")

	//fetch_domain, err := getDomain(top_level)
	//if err != nil {
	//		panic(err.Error())
	//	}
	//	domains = append(domains, fetch_domain)
	var realDomain Domain
	for _, d := range domains {
		// Create regex string to match against per-domain
		regexString := fmt.Sprintf(".*%s.*", d.Name)
		match, _ := regexp.MatchString(regexString, top_level)
		//fmt.Println(top_level)
		if match {
			fmt.Println(top_level)
			realDomain = d
		}
	}

	if (Domain{}) == realDomain {
		// if we dont know this domain, bail and return an empty set
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(""),
		})
	} else {
		// Domain matches, we should continue to search
		var device Record
		for i, j := range records {
			if j.DomainID != realDomain.ID {
				continue
			}
			if records[i].Name == strings.Split(domain, ".")[0] {
				device = records[i]
				fmt.Println("Record found in cache")
			}
		}

		if (Record{}) == device {
			//No existing records found in local cache, perform sql lookup
			// if the sql lookup fails then we give up
			device, _ = getRecordFromHost(strings.Split(domain, ".")[0], realDomain.ID)

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
	cfg, err := ini.Load("config.ini")
	if err != nil {
		panic(err.Error())
	}

	dbHost := cfg.Section("database").Key("host").String()
	dbUser := cfg.Section("database").Key("user").String()
	dbPass := cfg.Section("database").Key("pass").String()
	dbPort, _ := cfg.Section("database").Key("port").Int()
	dbName := cfg.Section("database").Key("database").String()

	err = dbConnect(dbUser, dbPass, dbHost, dbPort, dbName)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Populating data")
	populateData()
	fmt.Println("Done.")

	go func() {
		r := http.NewServeMux()
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
		r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))

		http.ListenAndServe(":6060", r)
	}()

	// Clean up records that exceed their TTL
	go func() {
		for {
			if err := cleanCache(); err != nil {
				log.Fatalf("Unable to clean up cache %s\n", err.Error())
			}
			time.Sleep(time.Second)
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
