package gpt3

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	gogpt "github.com/sashabaranov/go-gpt3"
)

const apiKey = "sk-Tv4pAfYXuCg6gbtXbLVviabjnROT8RlZ9ZZlSchz"

func Respond(ctx context.Context, inputs []string, dur time.Duration) (string, error) {
	// Add 10 seconds to generate one last sentence that we will crop.
	dur = dur + 10*time.Second
	c := gogpt.NewClient(apiKey)

	var buffer bytes.Buffer
	for _, i := range inputs {
		buffer.WriteString(fmt.Sprintf("Text: %s\n\n", i))
	}

	req := gogpt.CompletionRequest{
		Prompt:           fmt.Sprintf("%sText:", buffer.String()),
		MaxTokens:        int(3.6 * dur.Seconds()),
		Temperature:      0.8,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0.6,
		Stop:             []string{"Text:"},
	}

	log.Printf("Sending request %+v", req)
	resp, err := c.CreateCompletion(ctx, "davinci", req)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(resp.Choices[0].Text)
	log.Printf("Got response: %s", text)
	return text, nil
}
