package ch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	retry "github.com/avast/retry-go"
)

type inviteReq struct {
	Channel string `json:"channel"`
	UserID  int64  `json:"user_id"`
}

type apiResponse struct {
	Success bool `json:"success"`
}

func (c *Clubhouse) canMakeRequests() error {
	if c.RequestHeaders["CH-UserID"] == "" {
		return fmt.Errorf("CH-UserID header is not set")
	}
	if c.ChannelID == "" {
		return fmt.Errorf("ChannelID not set")
	}
	return nil
}

func (c *Clubhouse) Invite(ctx context.Context, user int64, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := retry.Do(func() error { return c.SpeakerRequest("invite_speaker", user) }, retry.Attempts(3)); err != nil {
		return fmt.Errorf("could not invite speaker: %v", err)
	}
	for {
		c.mu.Lock()
		if u, ok := c.Users[user]; !ok || u.Profile.IsSpeaker {
			if ok {
				c.Users[user].RaisedHand = false
			}
			c.mu.Unlock()
			break
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			c.Users[user].RaisedHand = false
			return fmt.Errorf("Invitiation for user %d expired", user)
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
}

func (c *Clubhouse) Uninvite(ctx context.Context, user int64) error {
	if err := retry.Do(func() error { return c.SpeakerRequest("uninvite_speaker", user) }, retry.Attempts(3)); err != nil {
		return fmt.Errorf("could not uninvite speaker: %v", err)
	}
	for {
		c.mu.Lock()
		if u, ok := c.Users[user]; !u.Profile.IsSpeaker || !ok {
			c.mu.Unlock()
			break
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return fmt.Errorf("Invite(): context timed out")
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
}

func (c *Clubhouse) UninviteAll(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for _, user := range c.Speakers() {
		log.Printf("Uninviting user %d", user)
		if err := c.Uninvite(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

func (c *Clubhouse) SpeakerRequest(method string, user int64) error {
	if method != "invite_speaker" && method != "uninvite_speaker" {
		return fmt.Errorf("unexpected method: %s", method)
	}
	if err := c.canMakeRequests(); err != nil {
		return fmt.Errorf("cannot make requests: %v", err)
	}

	postBody, err := json.Marshal(&inviteReq{Channel: c.ChannelID, UserID: user})
	if err != nil {
		return fmt.Errorf("could not serialize post body: %v", err)
	}

	url := fmt.Sprintf("https://www.clubhouseapi.com/api/%s", method)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postBody))
	if err != nil {
		return fmt.Errorf("could not create request: %v", err)
	}
	for k, v := range c.RequestHeaders {
		req.Header[k] = []string{v}
	}
	req.Header["Content-Type"] = []string{"application/json; charset=utf-8"}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read body: %v", err)
	}

	var r apiResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("could not unmarshal response: %v", err)
	}

	if !r.Success {
		return fmt.Errorf("unsuccessful response: %s", string(body))
	}
	return nil
}
