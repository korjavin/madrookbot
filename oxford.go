package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const (
	baseURL = "https://od-api.oxforddictionaries.com/api/v1"
)

var (
	appID  = os.Getenv("OXFORD_ID")
	appKey = os.Getenv("OXFORD_KEY")
)

type response struct {
	Metadata struct {
		Provider string `json:"provider"`
	} `json:"metadata"`
	Results []struct {
		ID             string `json:"id"`
		Language       string `json:"language"`
		LexicalEntries []struct {
			Entries []struct {
				Etymologies         []string `json:"etymologies"`
				GrammaticalFeatures []struct {
					Text string `json:"text"`
					Type string `json:"type"`
				} `json:"grammaticalFeatures"`
				Senses []struct {
					Definitions []string `json:"definitions"`
					Domains     []string `json:"domains"`
					Examples    []struct {
						Text string `json:"text"`
					} `json:"examples"`
					ID        string   `json:"id"`
					Registers []string `json:"registers"`
				} `json:"senses"`
			} `json:"entries"`
			Language        string `json:"language"`
			LexicalCategory string `json:"lexicalCategory"`
			Pronunciations  []struct {
				AudioFile        string   `json:"audioFile,omitempty"`
				Dialects         []string `json:"dialects"`
				PhoneticNotation string   `json:"phoneticNotation"`
				PhoneticSpelling string   `json:"phoneticSpelling"`
			} `json:"pronunciations"`
			Text string `json:"text"`
		} `json:"lexicalEntries"`
		Type string `json:"type"`
		Word string `json:"word"`
	} `json:"results"`
}

func getOxfordDefinition(word string) (answer string) {
	url := baseURL + "/entries/en/" + word
	bytes, err := getOxfordEndpoint(url)
	if err != nil {
		return ""
	}
	var r response
	if err = json.Unmarshal(bytes, &r); err != nil {
		return ""
	}
	answer = "Oxford definition(s) for " + word + " :"
	for _, v := range r.Results {
		answer += "\n"
		for _, l := range v.LexicalEntries {
			for _, e := range l.Entries {
				for _, s := range e.Senses {
					for _, d := range s.Definitions {
						answer += "- " + d + "\n"
					}
					for _, e := range s.Examples {
						answer += "Example: " + e.Text + "\n"
					}
				}
			}

			for _, p := range l.Pronunciations {
				answer += "Audio: " + p.AudioFile + "\n"
				break
			}
		}
	}
	return answer
}

func getOxfordEndpoint(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("app_id", appID)
	req.Header.Set("app_key", appKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("body close : %v \n", err)
		}
	}()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("[ERROR] %v", err)
	}
	return body, nil
}
