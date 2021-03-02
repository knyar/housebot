package capture

import (
	"bytes"
	"context"
	"io"
	"log"
	"sort"
	"time"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

var (
	idleTimeout   = 5 * time.Second
	idleThreshold = 20 // -20dB or quieter
)

func (c *Capturer) Capture(ctx context.Context, done <-chan struct{}) (string, error) {
	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:                   speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz:            16000,
					LanguageCode:               "en-US",
					EnableAutomaticPunctuation: true,
					ProfanityFilter:            false,
					Model:                      "phone_call",
					UseEnhanced:                true,
					Metadata: &speechpb.RecognitionMetadata{
						InteractionType:     speechpb.RecognitionMetadata_PHONE_CALL,
						OriginalMediaType:   speechpb.RecognitionMetadata_AUDIO,
						RecordingDeviceType: speechpb.RecognitionMetadata_PHONE_LINE,
					},
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	sound, volume, cancel := c.Consume()

	// Cancel when done is closed.
	go func() {
		<-done
		cancel()
	}()

	// Track volume
	go func() {
		lastNotIdle := time.Now()
		var volumes []float64
		for vol := range volume {
			volumes = append(volumes, vol)
			if int(vol) < idleThreshold {
				lastNotIdle = time.Now()
			}
			if time.Now().Sub(lastNotIdle) > idleTimeout {
				log.Printf("Idle for %s; cancelling.", time.Now().Sub(lastNotIdle))
				cancel()
			}
		}
		sort.Float64s(volumes)
		log.Printf("Captured volumes: min %f, max %f, p50 %f, p10 %f",
			volumes[0], volumes[len(volumes)-1], volumes[len(volumes)/2], volumes[len(volumes)/10])
	}()

	// Read audio stream.
	go func() {
		len := 0
		for buf := range sound {
			len = len + 1
			if err := stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
					AudioContent: buf,
				},
			}); err != nil {
				log.Printf("Could not send audio: %v", err)
			}
		}
		log.Printf("Sent %d chunks of audio to the speach API", len)
		if err := stream.CloseSend(); err != nil {
			log.Fatalf("Could not close stream: %v", err)
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
