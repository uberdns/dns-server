package main

import (
	"database/sql"
	"fmt"
	"log"
)

func dbConnect(username string, password string, host string, port int, database string) error {
	conn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, database)
	debugMsg("Connecting to " + host)
	dbc, err := sql.Open("mysql", conn)

	if err != nil {
		log.Fatal(err)
	}
	debugMsg("Connected to " + host)

	//defer dbc.Close()

	err = dbc.Ping()
	if err != nil {
		log.Fatal(err)
	}

	debugMsg("DB Ping successful")

	dbConn = *dbc
	return nil
}
