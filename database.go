package main

import (
	"database/sql"
	"fmt"

	log "github.com/sirupsen/logrus"
)

func dbConnect(username string, password string, host string, port int, database string) error {
	conn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, database)
	log.Info("Connecting to " + host)
	dbc, err := sql.Open("mysql", conn)

	if err != nil {
		return err
	}
	log.Info("Connected to " + host)

	//defer dbc.Close()

	err = dbc.Ping()
	if err != nil {
		return err
	}

	log.Info("DB Ping successful")

	dbConn = *dbc
	return nil
}
