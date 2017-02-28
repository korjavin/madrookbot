package main

import (
	"github.com/mrjones/oauth"
	"log"
	"os"
)

var (
	aboomkey    string
	aboomsecret string
)

func init() {
	aboomkey = os.Getenv("ABOOM_KEY")
	aboomsecret = os.Getenv("ABOOM_SECRET")
}
func uploadFile(filename string) error {
	c := oauth.NewConsumer(
		aboomkey,
		aboomsecret,
		oauth.ServiceProvider{
			RequestTokenUrl:   "http://api.audioboo.fm/oauth/request_token",
			AuthorizeTokenUrl: "http://api.audioboo.fm/oauth/authorize",
			AccessTokenUrl:    "http://api.audioboo.fm/oauth/access_token",
		})

	// c.Debug(true)
	requestToken, u, err := c.GetRequestTokenAndUrl("oob")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("(1) Go to: " + u)
	log.Println("(2) Grant access, you should get back a verification code.")
	log.Println("(3) Enter that verification code here: ")

	verificationCode := ""
	//log.Scanln(&verificationCode)

	accessToken, err := c.AuthorizeToken(requestToken, verificationCode)
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.MakeHttpClient(accessToken)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
