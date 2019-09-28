// To-Do:
// - prometheus exporter
// - promtool
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/miekg/dns"
	"gopkg.in/ini.v1"
)

// Domain - struct for storing information regarding domains
type Domain struct {
	ID   int64
	Name string
}

// Record -- struct for storing information regarding records
type Record struct {
	ID       int
	Name     string
	IP       string
	TTL      int64     //TTL for caching
	Created  time.Time //datetime the record created in database
	DOB      time.Time //time record created, used for cache expiry
	DomainID int64
}

// CacheControlMessage -- struct for storing/parsing redis cache control messages
//  					  from the api server
type CacheControlMessage struct {
	Action string
	Type   string
	Object string
}

var domains = []Domain{}
var records = []Record{}

var redisClient *redis.Client
var redisCacheChannelName string
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

func getRecordFromHost(host string, domainID int64) (Record, error) {
	var record Record

	query := "SELECT id, name, ip_address, ttl, domain_id FROM dns_record WHERE name = ? AND domain_id = ?"
	dq, err := dbConn.Prepare(query)

	if err != nil {
		log.Fatal(err)
	}

	defer dq.Close()

	err = dq.QueryRow(host, domainID).Scan(&record.ID, &record.Name, &record.IP, &record.TTL, &record.DomainID)
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

func (fuck *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	domain := msg.Question[0].Name
	msg.Authoritative = true

	cleanDomain := strings.TrimRight(domain, ".")
	domainSplit := strings.Split(cleanDomain, ".")
	// Capture domain name plus TLD
	topLevelDomain := strings.Join(domainSplit[1:], ".")
	subdomain := domainSplit[0]

	var realDomain Domain
	for _, d := range domains {
		if topLevelDomain == d.Name {
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
				fmt.Println("Non-cached record, adding to cache")
				device.DOB = time.Now()
				records = append(records, device)
			}

		}

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

func addRecordToCache(record Record) error {
	// if record already exists in cache, do nothing
	for _, i := range records {
		if i.ID == record.ID {
			return nil
		}
	}
	// Set DOB to now as we're creating the object in cache
	record.DOB = time.Now()
	records = append(records, record)
	return nil
}

func removeRecordFromCache(record Record) error {
	for i := range records {
		if records[i].ID == record.ID {
			records = append(records[:i], records[i+1:]...)
		}
	}
	return nil
}

// Run through record objects currently cached and evaluate
// whether we need to expire them (remove)
func cleanCache() error {
	// this is a terrible way to do this, however i couldnt find a
	// great way to prevent "rebuilding" the list of records without
	// yanking entries from underneath it and subsequently causing
	// indexing/sorting errors and deleting items at wrong index
	var newRecords []Record
	fmt.Println(len(records))
	for i := range records {
		if (time.Now().Unix() - records[i].DOB.Unix()) < records[i].TTL {
			newRecords = append(newRecords, records[i])
		}
		fmt.Println(len(newRecords))
	}
	// copy newRecords to records
	//copy(records, newRecords)
	records = newRecords

	return nil
}

func cacheMessageHandler(msg CacheControlMessage) error {
	switch strings.ToLower(msg.Type) {
	case "record":
		var record Record
		json.Unmarshal([]byte(msg.Object), &record)

		//Record cache manage routes
		switch strings.ToLower(msg.Action) {
		case "create":
			addRecordToCache(record)
		case "purge":
			removeRecordFromCache(record)
		}
	case "domain":
		var domain Domain
		json.Unmarshal([]byte(msg.Object), &domain)

		//Domain cache manage routes
		switch strings.ToLower(msg.Action) {
		case "create":
			//Create the domain object and then throw it into cache
		case "purge":
			//Purge the domain object from cache
		}
	}
	return nil
}

// Watch for redis messages in the cache purge channel
// when one comes in, remove the record from the cache
func watchCacheChannel(rdc *redis.PubSub) {
	defer rdc.Close()
	log.Println("Watching for redis cache management messages...")
	ch := rdc.Channel()

	for msg := range ch {
		var cacheMsg CacheControlMessage
		json.Unmarshal([]byte(msg.Payload), &cacheMsg)
		// we can run this async without caring about returning a result
		// this is just "we have a record, give cacheMessageHandler() the msg
		// and move on with the next msg"
		go cacheMessageHandler(cacheMsg)
	}
}

func main() {
	var (
		recordCacheDepthCounter = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "record_cache_depth",
			Help: "Number of records stored in cache",
		})
		domainCacheDepthCounter = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "domain_cache_depth",
			Help: "Number of domains stored in cache",
		})
	)
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

	cfg, err := ini.Load("config.ini")
	if err != nil {
		panic(err.Error())
	}

	dbHost := cfg.Section("database").Key("host").String()
	dbUser := cfg.Section("database").Key("user").String()
	dbPass := cfg.Section("database").Key("pass").String()
	dbPort, _ := cfg.Section("database").Key("port").Int()
	dbName := cfg.Section("database").Key("database").String()

	redisHost := cfg.Section("redis").Key("host").String()
	redisPassword := cfg.Section("redis").Key("password").String()
	redisDB, _ := cfg.Section("redis").Key("db").Int()
	redisCacheChannelName = cfg.Section("redis").Key("cache_channel").String()

	prometheusPort := cfg.Section("prometheus").Key("port").String()

	err = dbConnect(dbUser, dbPass, dbHost, dbPort, dbName)
	if err != nil {
		panic(err.Error())
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Ping/Pong - (Will be) Used for health check
	go func() {
		for {
			_, err = redisClient.Ping().Result()
			if err != nil {
				fmt.Println("Unable to communicate with Redis at ", redisHost)
				fmt.Println(err.Error())
			}
			time.Sleep(time.Second)
		}
	}()

	// Listen for cache clean messages from redis
	go func() {
		fmt.Println("Subscribing to ", redisCacheChannelName)
		pubsub := redisClient.Subscribe(redisCacheChannelName)
		watchCacheChannel(pubsub)
	}()

	// Start prometheus metrics
	go func() {
		prometheus.MustRegister(recordCacheDepthCounter)
		prometheus.MustRegister(domainCacheDepthCounter)
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", prometheusPort), nil))
	}()

	fmt.Println("Populating data")
	populateData()
	fmt.Println("Done.")

	// Clean up records that exceed their TTL
	go func() {
		for {
			if err := cleanCache(); err != nil {
				log.Fatalf("Unable to clean up cache %s\n", err.Error())
			}
			recordCacheDepthCounter.Set(float64(len(records)))
			domainCacheDepthCounter.Set(float64(len(domains)))
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
