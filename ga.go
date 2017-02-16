package main

import (
	"github.com/jpillora/go-ogle-analytics"
	"log"
	"os"
)

var (
	clientID string
	client   *ga.Client
)

func init() {
	var err error
	clientID = os.Getenv("GA_ID")
	client, err = ga.NewClient(clientID)
	if err != nil {
		log.Printf("ga connect: %v \n", err)
	}
}

func sendEvent(category, action, label string) {
	if client == nil {
		return
	}
	err := client.Send(ga.NewEvent(category, action).Label(label))
	if err != nil {
		log.Printf("ga send: %v \n", err)
	}
}
