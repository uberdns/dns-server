package main

import (
	log "github.com/sirupsen/logrus"
  )

func logger(service string) *log.Entry {
	retLog := log.WithFields(log.Fields{
		"service": service,
	})
	return retLog
}