package voice

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	texttospeechpb "google.golang.org/genproto/googleapis/cloud/texttospeech/v1"
)

func Say(ctx context.Context, device string, content string) error {
	filename, err := tts(ctx, content)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "gst-launch-1.0", "-q",
		"filesrc", fmt.Sprintf("location=%s", filename), "!", "decodebin",
		"!", "audioconvert", "!", "audioresample", "!", device)
	log.Printf("Playing response: %s", strings.Join(cmd.Args, " "))
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error while playing response: %s", out)
	} else {
		log.Printf("Finished playing")
	}
	return err
}

func tts(ctx context.Context, content string) (string, error) {
	hash := fmt.Sprintf("%x", sha1.Sum([]byte(content)))
	filename := fmt.Sprintf("cache/%s.ogg", hash)

	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		return filename, nil
	}

	client, err := texttospeech.NewClient(ctx)
	if err != nil {
		return "", err
	}
	req := texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: content},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-AU",
			Name:         "en-AU-Standard-D",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_MALE,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_OGG_OPUS,
			SpeakingRate:  0.8,
		},
	}

	resp, err := client.SynthesizeSpeech(ctx, &req)
	if err != nil {
		return "", err
	}

	// The resp's AudioContent is binary.
	err = ioutil.WriteFile(filename, resp.AudioContent, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Audio content written to file: %v\n", filename)

	return filename, nil
}
