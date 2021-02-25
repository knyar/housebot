package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/knyar/housebot/capture"
	"github.com/knyar/housebot/ch"
	"github.com/knyar/housebot/gpt3"
	"github.com/knyar/housebot/voice"
)

var stripSentence = regexp.MustCompile(`(.*\.).*`)

func main() {
	ctx := context.Background()

	ch, err := ch.New("/Users/ryzh/code/housebot/mitmproxy.log")
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/ch", ch.HttpRoot)
	log.Println("Listening 8080")
	http.ListenAndServe(":8080", nil)

	captured, err := capture.Capture(ctx, 90*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	// Add a dot at the end if it does not exist.
	captured = fmt.Sprintf("%s.", strings.TrimSuffix(captured, "."))

	resp, err := gpt3.Respond(ctx, captured, 90*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	// Strip last sentence that is likely to be incomplete.
	resp = stripSentence.ReplaceAllString(resp, "$1")

	// fmt.Println(resp)
	err = voice.Say(ctx, resp)
	if err != nil {
		log.Fatal(err)
	}
}
