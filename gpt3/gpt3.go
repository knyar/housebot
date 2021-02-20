package gpt3

import (
	"context"
	"fmt"
	"log"
	"time"

	gogpt "github.com/sashabaranov/go-gpt3"
)

const apiKey = "sk-Tv4pAfYXuCg6gbtXbLVviabjnROT8RlZ9ZZlSchz"

func Respond(ctx context.Context, input string, dur time.Duration) (string, error) {
	// Add 10 seconds to generate one last sentence that we will crop.
	dur = dur + 10*time.Second
	log.Printf("Creating GPT client")
	c := gogpt.NewClient(apiKey)
	prompt := "This is a transcript of a philosophical podcast."
	req := gogpt.CompletionRequest{
		Prompt:           fmt.Sprintf("%s\n\n%s\n\n", prompt, input),
		MaxTokens:        int(3.6 * dur.Seconds()),
		Temperature:      0.8,
		FrequencyPenalty: 0.8,
		PresencePenalty:  0.5,
	}

	log.Printf("Sending request %+v", req)
	resp, err := c.CreateCompletion(ctx, "davinci", req)
	if err != nil {
		return "", err
	}
	text := resp.Choices[0].Text
	log.Printf("Got response: %s", text)
	return text, nil
}
