package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	"github.com/knyar/housebot/ch"
)

var stripSentence = regexp.MustCompile(`(.*\.).*`)

func main() {
	ctx := context.Background()

	ch, err := ch.New("/var/log/mitmproxy.log")
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/ch", ch.HttpRoot)
	log.Println("Listening 9090")
	go func() { log.Fatal(http.ListenAndServe(":9090", nil)) }()

	// Catch up with the log.
	time.Sleep(1 * time.Second)

	for {
		if err := ch.UninviteAll(ctx, 5*time.Second); err != nil {
			log.Printf("ERROR while uninviting all: %v", err)
		}

		users := ch.Candidates()
		if len(users) == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		idx := rand.Int63n(int64(len(users)))
		u := users[idx]
		if err := ch.Invite(ctx, u, 5*time.Second); err != nil {
			log.Printf("ERROR while inviting user %d: %v", u, err)
			err = ch.SpeakerRequest("uninvite_speaker", u)
			log.Printf("Tried to uninvite user %d: %v", u, err)
			continue
		}

		wait := 60 * time.Second
		log.Printf("Sleeping for %v", wait)
		deadline := time.Now().Add(wait)
		for deadline.Sub(time.Now()) > 0 {
			if user := ch.User(u); user == nil || !user.Profile.IsSpeaker {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}
