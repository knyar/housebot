package ch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
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
		return fmt.Errorf("unsuccessful response: %+v", body)
	}
	return nil
}
