package capture

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Capturer struct {
	consumers map[int64]*consumer
	mu        sync.RWMutex
}

type consumer struct {
	sound  chan []byte
	volume chan float64
	file   *os.File
}

func NewCapturer(ctx context.Context, device string) (*Capturer, error) {
	c := &Capturer{consumers: make(map[int64]*consumer)}

	args := []string{"-m"}
	args = append(args, strings.Split(device, " ")...)
	args = append(args, "!", "identity",
		"!", "level",
		"!", "queue", "max-size-time=50000000", // 50ms
		"!", "audioconvert", "!", "audio/x-raw,format=S16LE,channels=1,rate=16000",
		"!", "filesink", "buffer-mode=2", "buffer-size=1024", "location=/dev/stderr")

	cmd := exec.CommandContext(ctx, "gst-launch-1.0", args...)
	log.Printf("Running %s", strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	go c.consumeVolume(stdout)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	go c.consumeSound(stderr)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("ERROR while running capturer command: %v", err)
		}
	}()

	return c, nil
}

func (c *Capturer) Consume() (<-chan []byte, <-chan float64, func()) {
	sound := make(chan []byte, 5)
	volume := make(chan float64, 5)

	id := rand.Int63()
	c.mu.Lock()
	for ; ; id = rand.Int63() {
		if _, ok := c.consumers[id]; !ok {
			filename := fmt.Sprintf("data/recording/%s.%d.raw", time.Now().Format(time.RFC3339), id)
			file, err := os.Create(filename)
			if err != nil {
				log.Fatal(err)
			}
			c.consumers[id] = &consumer{sound: sound, volume: volume, file: file}
			break
		}
	}
	c.mu.Unlock()

	cancel := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if consumer, ok := c.consumers[id]; ok {
			close(consumer.sound)
			close(consumer.volume)
			consumer.file.Close()
			delete(c.consumers, id)
		}
	}

	return sound, volume, cancel
}

func (c *Capturer) consumeVolume(p io.ReadCloser) {
	scanner := bufio.NewScanner(p)
	levelParser := regexp.MustCompile(`peak=\(GValueArray\)< -([0-9.]+) >`)
	for scanner.Scan() {
		m := levelParser.FindStringSubmatch(scanner.Text())
		if len(m) > 1 {
			volume, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				log.Fatal(err)
			}
			c.mu.RLock()
			for _, consumer := range c.consumers {
				consumer.volume <- volume
			}
			c.mu.RUnlock()
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func (c *Capturer) consumeSound(p io.ReadCloser) {
	buf := make([]byte, 1024)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			c.mu.RLock()
			for _, consumer := range c.consumers {
				consumer.sound <- buf[:n]
				consumer.file.Write(buf[:n])
			}
			c.mu.RUnlock()
		}
		if err != nil {
			log.Panicf("ERROR: capturer could not read from stdin: %v", err)
		}
	}
}
