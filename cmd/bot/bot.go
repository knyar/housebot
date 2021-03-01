package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/knyar/housebot/capture"
	"github.com/knyar/housebot/ch"
	"github.com/knyar/housebot/gpt3"
	"github.com/knyar/housebot/voice"
)

var stripSentence = regexp.MustCompile(`(?s)(.*\.).*`)
var thanks = []string{
	"Hmm. Thank you, %s.",
	"OK. Your time is up, %s. Thank you.",
	"Alright... Thanks a lot, %s.",
}

func main() {
	stageTime := flag.Duration("stage_time", 60*time.Second, "how long each speaker gets on stage")
	mitmLog := flag.String("mitm_log", "/var/log/mitmproxy.log", "path to mitmdump-generated log of Clubhouse traffic")
	soundIn := flag.String("sound_in", "alsasrc", "gstreamer input")
	soundOut := flag.String("sound_out", "autoaudiosink", "gstreamer output")
	responseFrequncy := flag.Int("response_frequency", 3, "respond after every X humans")
	flag.Parse()

	ctx := context.Background()

	ch, err := ch.New(*mitmLog)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/ch", ch.HttpRoot)
	log.Println("Listening 9090")
	go func() { log.Fatal(http.ListenAndServe(":9090", nil)) }()

	// Catch up with the log.
	time.Sleep(1 * time.Second)

	if err := ch.UninviteAll(ctx, 5*time.Second); err != nil {
		log.Printf("ERROR while uninviting all: %v", err)
	}

	capturer, err := capture.NewCapturer(ctx, *soundIn)
	if err != nil {
		log.Fatal(err)
	}

	responses := []string{
		// "Just a reminder. The rules of this room are simple. Each speaker gets the stage for one minute; next speaker is chosen randomly amongst people who raised their hand. Thanks for joining us.",
	}

	humansSpoken := 0
	var humanText []string
	for {
		if len(responses) > 0 {
			for _, resp := range responses {
				err = voice.Say(ctx, *soundOut, resp)
				if err != nil {
					log.Fatal(err)
				}
			}
			responses = nil
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

		humansSpoken = humansSpoken + 1

		// Pre-fetch a 'thanks' response.
		resp := fmt.Sprintf(thanks[rand.Intn(len(thanks))], ch.User(u).Profile.FirstName)
		go func() { voice.Tts(ctx, resp) }()

		log.Printf("Capturing audio for %v", stageTime)
		captured := make(chan string, 1)
		done := make(chan struct{})
		go func() {
			c, err := capturer.Capture(ctx, done)
			if err != nil {
				log.Fatal(err)
			}
			captured <- c
		}()

		deadline := time.Now().Add(*stageTime)
		for deadline.Sub(time.Now()) > 0 {
			if user := ch.User(u); user == nil || !user.Profile.IsSpeaker {
				log.Printf("Speaker %d left early; cancelling recording", u)
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		close(done)

		if err := ch.UninviteAll(ctx, 5*time.Second); err != nil {
			log.Printf("ERROR while uninviting all: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			err = voice.Say(ctx, *soundOut, resp)
			if err != nil {
				log.Fatal(err)
			}
			wg.Done()
		}()

		c := <-captured
		c = fmt.Sprintf("%s.", strings.TrimSuffix(c, "."))
		humanText = append(humanText, c)

		if len(humanText) >= *responseFrequncy || len(ch.Candidates()) == 0 {
			resp, err := gpt3.Respond(ctx, humanText, *stageTime)
			if err != nil {
				log.Fatal(err)
			}
			// Strip last sentence that is likely to be incomplete.
			resp = stripSentence.ReplaceAllString(resp, "$1")
			responses = append(responses, resp)
			humanText = nil
		}

		wg.Wait()
	}
}
