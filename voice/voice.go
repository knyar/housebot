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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	texttospeechpb "google.golang.org/genproto/googleapis/cloud/texttospeech/v1"
)

func Say(ctx context.Context, device string, content string) error {
	filename, err := Tts(ctx, content)
	if err != nil {
		return err
	}
	args := []string{"-q",
		"filesrc", fmt.Sprintf("location=%s", filename), "!", "decodebin",
		"!", "audioconvert", "!", "audioresample", "!", "pitch", "pitch=0.95",
		"!"}
	args = append(args, strings.Split(device, " ")...)
	cmd := exec.CommandContext(ctx, "gst-launch-1.0", args...)
	log.Printf("Playing response: %s (%s)", strings.Join(cmd.Args, " "), content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error while playing response: %s", out)
	}
	return nil
}

func Tts(ctx context.Context, content string) (string, error) {
	hash := fmt.Sprintf("%x", sha1.Sum([]byte(content)))
	filename := fmt.Sprintf("data/tts/%s.ogg", hash)

	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		return filename, nil
	}

	data, err := AmazonTts(ctx, content)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Audio content written to file: %v\n", filename)

	return filename, nil
}

func AmazonTts(ctx context.Context, content string) ([]byte, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String("eu-central-1")},
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := polly.New(sess)
	input := &polly.SynthesizeSpeechInput{
		Engine:       aws.String("neural"),
		LanguageCode: aws.String("en-GB"),
		OutputFormat: aws.String("ogg_vorbis"),
		Text:         aws.String(content),
		VoiceId:      aws.String("Brian"),
	}

	output, err := svc.SynthesizeSpeech(input)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(output.AudioStream)
}

func GoogleTts(ctx context.Context, content string) ([]byte, error) {
	client, err := texttospeech.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	req := texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: content},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-AU",
			Name:         "en-AU-Wavenet-D",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_MALE,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_OGG_OPUS,
			SpeakingRate:  0.75,
			Pitch:         -6,
		},
	}

	resp, err := client.SynthesizeSpeech(ctx, &req)
	if err != nil {
		return nil, err
	}

	return resp.AudioContent, nil
}
