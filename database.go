package main

import (
	"database/sql"
	"fmt"
)

func dbConnect(username string, password string, host string, port int, database string) error {
	conn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, database)
	logger("db").Info("Connecting to " + host)
	dbc, err := sql.Open("mysql", conn)

	if err != nil {
		return err
	}
	logger("db").Info("Connected to " + host)

	err = dbc.Ping()
	if err != nil {
		return err
	}

	logger("db").Debug("DB Ping successful")

	dbConn = *dbc
	return nil
}
