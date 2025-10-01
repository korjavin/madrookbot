package main

import (
	"log"
	"net/http"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func getDoc(url string) (*html.Node, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[ERROR]  %v\n", err)
	}
	req.Header.Set("Accept-Language", "en-US;q=0.8,en;q=0.6")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.100 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			log.Printf("body close %v \n", err)
		}
	}()
	root, err := html.Parse(res.Body)
	if err != nil {
		return nil, err
	}
	return root, nil
}

func getIdiom(term string) string {
	text := ""
	term = regexp.MustCompile(`\s+`).ReplaceAllLiteralString(term, "+")
	get, err := getDoc("http://idioms.thefreedictionary.com/" + term)
	if err != nil {
		log.Println(err)
	} else {
		doc := goquery.NewDocumentFromNode(get)
		doc.Find(".ds-single").Each(func(i int, s *goquery.Selection) {
			text += s.Text() + "\n"
		})
		if text == "" {
			doc.Find("#Definition > section:nth-child(1)").Each(func(i int, s *goquery.Selection) {
				text += s.Text() + "\n"
			})
		}

	}
	return text
}
