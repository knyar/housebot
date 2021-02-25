package ch

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/hpcloud/tail"
)

var recordHeaders = []string{"Authorization", "Accept-Language", "CH-Languages", "CH-UserID", "CH-Locale", "CH-AppBuild", "CH-AppVersion", "CH-DeviceId", "User-Agent"}

type User struct {
	Profile    *pubnubUser
	RaisedHand bool
}
type Clubhouse struct {
	logfile        *tail.Tail
	LastTime       time.Time
	RequestHeaders map[string]string
	UserID         string
	ChannelID      string
	Users          map[int64]*User
	tpl            *template.Template
	mu             sync.Mutex
}

func New(logfile string) (*Clubhouse, error) {
	var err error
	c := &Clubhouse{
		RequestHeaders: make(map[string]string),
		Users:          make(map[int64]*User),
	}
	c.tpl, err = template.ParseFiles("ch/index.html")
	if err != nil {
		return nil, err
	}

	c.logfile, err = tail.TailFile(logfile, tail.Config{ReOpen: true, MustExist: true, Follow: true})
	if err != nil {
		return nil, err
	}

	go c.run()
	return c, nil
}

func (c *Clubhouse) run() {
	headers := recordHeadersMap()
	for line := range c.logfile.Lines {
		var msg logMessage
		if err := json.Unmarshal([]byte(line.Text), &msg); err != nil {
			log.Printf("ERROR unmarshaling ch log: %v", err)
			continue
		}
		c.mu.Lock()
		sec, dec := math.Modf(msg.Ts)
		c.LastTime = time.Unix(int64(sec), int64(dec*(1e9)))

		if msg.Request.Headers["Host"] == "clubhouse.pubnub.com" || msg.Request.Headers["Host"] == "clubhouse.pubnubapi.com" {
			c.updateIDs(&msg)
			c.updateUsers(&msg)
		}

		if msg.Request.Headers["Host"] == "www.clubhouseapi.com" {
			for k, v := range msg.Request.Headers {
				if headers[k] {
					c.RequestHeaders[k] = v
				}
			}
		}
		c.mu.Unlock()
	}
	if err := c.logfile.Wait(); err != nil {
		log.Printf("ERROR: tail: %v", err)
	}
}

func (c *Clubhouse) HttpRoot(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	if action, ok := params["action"]; ok {
		if action[0] == "invite" || action[0] == "uninvite" {
			if user, ok := params["user"]; ok {
				userID, err := strconv.ParseInt(user[0], 10, 64)
				if err != nil {
					log.Printf("ERROR: could not parse invite user_id from: %+v", params)
				} else {
					method := fmt.Sprintf("%s_speaker", action[0])
					if err := c.SpeakerRequest(method, userID); err != nil {
						log.Printf("ERROR: could not %s user %d: %v", action[0], userID, err)
					}
					log.Printf("Speaker %sd: %d", action[0], userID)
					http.Redirect(w, req, req.URL.Path, http.StatusFound)
					return
				}
			}
		}
	}
	c.tpl.Execute(w, c)
}

func recordHeadersMap() map[string]bool {
	m := make(map[string]bool)
	for _, h := range recordHeaders {
		m[h] = true
	}
	return m
}
