package main

import "log"

func debugMsg(msg string) {
	if DEBUG {
		log.Println("[DEBUG] " + msg)
	}
}
