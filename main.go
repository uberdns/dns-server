//To-Do:
//  - cache + return cname records (currently returns cname on first lookup then returns cached A record)
//  - ttl returns constant value - should be decremented from cache DOB + time in cache
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/ini.v1"

	log "github.com/sirupsen/logrus"
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

type DomainMap struct {
	Domains map[int]Domain
	mu      *sync.Mutex
}

type RecordMap struct {
	Records    map[int]Record
	QueryCount int64
	mu         *sync.Mutex
}

var domains DomainMap
var records RecordMap

var domainChannel = make(chan Domain)
var recursiveDomainChannel = make(chan Domain)

var recursiveDomains DomainMap
var recursiveRecords RecordMap

var recordCacheChannel = make(chan Record)
var recordCachePurgeChannel = make(chan Record)
var recursiveCacheChannel = make(chan Record)
var recursiveCachePurgeChannel = make(chan Record)

// DEBUG var used for logging
var DEBUG = false

var redisClient *redis.Client
var redisCacheChannelName string
var dbConn sql.DB
var recordQueryCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "uberdns_record_query_total",
	},
	[]string{
		"type",
		"query",
	},
)

func main() {
	var (
		recordCacheDepthCounter = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uberdns_record_cache_depth",
			},
			[]string{
				"type",
			},
		)

		domainCacheDepthCounter = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "uberdns_domain_cache_depth",
			},
			[]string{
				"type",
			},
		)
	)

	domains.Domains = make(map[int]Domain)
	domains.mu = new(sync.Mutex)
	recursiveDomains.Domains = make(map[int]Domain)
	recursiveDomains.mu = new(sync.Mutex)

	records.Records = make(map[int]Record)
	records.mu = new(sync.Mutex)
	recursiveRecords.Records = make(map[int]Record)
	recursiveRecords.mu = new(sync.Mutex)

	cfgFile := flag.String("config", "config.ini", "Path to the config file")
	debug := flag.Bool("debug", false, "Toggle debug mode.")
	flag.Parse()

	DEBUG = *debug

	cfg, err := ini.Load(*cfgFile)
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

	prometheusPort := cfg.Section("dns").Key("prometheus_port").String()
	pprofPort, _ := cfg.Section("dns").Key("pprof_port").Int()
	logFilename := cfg.Section("dns").Key("log_file").String()

	// Start logger
	logFile, err := os.OpenFile(logFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)

	formatter := new(log.TextFormatter)
	formatter.FullTimestamp = true

	log.SetFormatter(formatter)
	if err != nil {
		log.Fatal(err)
	} else {
		log.SetOutput(logFile)
	}

	// Start pprof
	go func() {
		r := http.NewServeMux()
		r.HandleFunc("/debug/domain/", debugDomainHandler)
		r.HandleFunc("/debug/record/", debugRecordHandler)
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
		r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))

		http.ListenAndServe(fmt.Sprintf(":%d", pprofPort), r)
	}()

	err = dbConnect(dbUser, dbPass, dbHost, dbPort, dbName)
	if err != nil {
		log.Fatal(err.Error())
	}

	redisClient = redisConnect(redisHost, redisPassword, redisDB)

	// Listen for cache clean messages from redis
	go func() {
		watchCacheChannel(redisClient, redisCacheChannelName)
	}()

	// Start prometheus metrics
	go func() {
		prometheus.MustRegister(recordCacheDepthCounter)
		prometheus.MustRegister(domainCacheDepthCounter)
		prometheus.MustRegister(recordQueryCounter)
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", prometheusPort), nil))
	}()

	go watchCache(recursiveCacheChannel, recursiveCachePurgeChannel, recursiveRecords)
	go watchCache(recordCacheChannel, recordCachePurgeChannel, records)

	go domainChannelHandler(domainChannel, domains)
	go domainChannelHandler(recursiveDomainChannel, recursiveDomains)

	dp := make(chan bool, 1)
	populateData(dp)
	<-dp

	// Clean up records that exceed their TTL
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {

			recordCacheDepthCounter.WithLabelValues("uberdns").Set(float64(records.Count()))

			domainCacheDepthCounter.WithLabelValues("uberdns").Set(float64(domains.Count()))

			recordCacheDepthCounter.WithLabelValues("recurse").Set(float64(recursiveRecords.Count()))

			domainCacheDepthCounter.WithLabelValues("recurse").Set(float64(recursiveDomains.Count()))

		}
	}()

	go startListening("tcp", 53)
	go startListening("udp", 53)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping\n", s)

}
