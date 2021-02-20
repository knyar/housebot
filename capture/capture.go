package capture

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

func Capture(ctx context.Context, duration time.Duration) (string, error) {
	log.Printf("Creating speech client")
	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Creating stream")
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Configuring stream")
	// Send the initial configuration message.
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:                   speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz:            16000,
					LanguageCode:               "en-US",
					EnableAutomaticPunctuation: true,
					ProfanityFilter:            false,
					UseEnhanced:                true,
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "gst-launch-1.0", "-m", "osxaudiosrc", "device=103",
			"!", "identity", fmt.Sprintf("eos-after=%d", int(duration.Seconds()*100)),
			"!", "audioconvert", "!", "audioresample", "!", "level",
			"!", "audio/x-raw,format=S16LE,channels=1,rate=16000",
			"!", "filesink", "location=/dev/stderr")
		log.Printf("Running %s", strings.Join(cmd.Args, " "))
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			scanner := bufio.NewScanner(stdout)
			// buf := make([]byte, 1)
			// scanner.Buffer(buf, 1)
			levelParser := regexp.MustCompile(`peak=\(GValueArray\)< -([0-9.]+) >`)
			for scanner.Scan() {
				m := levelParser.FindStringSubmatch(scanner.Text())
				if len(m) > 1 {
					volume, err := strconv.ParseFloat(m[1], 64)
					if err != nil {
						log.Fatal(err)
					}
					fmt.Println(strings.Repeat("#", int(volume)))
				}
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
		}()

		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}

		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		log.Printf("Speak for %v\n", duration)

		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
						AudioContent: buf[:n],
					},
				}); err != nil {
					log.Printf("Could not send audio: %v", err)
				}
			}
			if err == io.EOF {
				if err := cmd.Wait(); err != nil && ctx.Err() != context.DeadlineExceeded {
					log.Fatalf("Wait() error: %v", err)
				}
				// Nothing else to pipe, close the stream.
				if err := stream.CloseSend(); err != nil {
					log.Fatalf("Could not close stream: %v", err)
				}
				return
			}
			if err != nil {
				log.Printf("Could not read from stdin: %v", err)
				continue
			}
		}
	}()

	var response bytes.Buffer
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			// Workaround while the API doesn't give a more informative error.
			if err.Code == 3 || err.Code == 11 {
				log.Print("WARNING: Speech recognition request exceeded limit of 60 seconds.")
			}
			log.Fatalf("Could not recognize: %v", err)
		}
		for _, result := range resp.Results {
			if result.IsFinal {
				response.WriteString(result.Alternatives[0].GetTranscript())
			}
			log.Printf("Result: %+v\n", result)
		}
	}
	return response.String(), nil
}
