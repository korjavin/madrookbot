package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func getFile(url string) (string, error) {
	tmpfile, err := ioutil.TempFile("./", "voice")
	if err != nil {
		log.Printf("File error: %v", err)
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(tmpfile, resp.Body)
	if err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}
