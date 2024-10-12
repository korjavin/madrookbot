package main

import (
	"log"
	"os"
)

func main() {
	// print all env
	for _, e := range os.Environ() {
		log.Println(e)
	}
	buddy := os.Getenv("GPT_BUDDY")
	if buddy == "" {
		log.Println("GPT_BUDDY is not set")
		return
	}
	if os.Getenv("GPT_TOKEN") != "" {
		log.Printf("GPT_TOKEN is set, using GPT-3")
		initGPT()
	}
	botGo(buddy)
}
