package ch

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type logMessage struct {
	Ts      float64 `json:"ts"`
	Request struct {
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		URL     string            `json:"url"`
		Text    string            `json:"text"`
	} `json:"request"`
	Response struct {
		Status  int               `json:"status_code"`
		Headers map[string]string `json:"headers"`
		Cookies map[string]string `json:"cookies"`
		Text    string            `json:"text"`
	} `json:"response"`
}

type pubnubUser struct {
	UserID    int64  `json:"user_id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	IsSpeaker bool   `json:"is_speaker"`
}

type pubnubMessage struct {
	M []struct {
		D struct {
			Action      string      `json:"action"`
			Channel     string      `json:"channel"`
			UserID      int64       `json:"user_id"`
			UserProfile *pubnubUser `json:"user_profile"`
		} `json:"d"`
	} `json:"m"`
}

var customLogger = log.New(os.Stderr, "", 0)

func l(ts time.Time, format string, args ...interface{}) {
	customLogger.SetPrefix(ts.Format("2006-01-02 15:04:05_"))
	customLogger.Printf(format, args...)
}

var reHeartbeat = regexp.MustCompile(`channel/channel_user.([^.]+)\.(\d+)/heartbeat`)

func (c *Clubhouse) updateIDs(m *logMessage) {
	if m := reHeartbeat.FindStringSubmatch(m.Request.URL); len(m) > 0 {
		var err error
		c.ChannelID = m[1]
		c.UserID, err = strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		if c.RequestHeaders["CH-UserID"] != "" && c.RequestHeaders["CH-UserID"] != m[2] {
			log.Printf("WARN: inconsistent user-id %s and %d", c.RequestHeaders["CH-UserID"], c.UserID)
		}
	}
}

func (c *Clubhouse) updateUsers(logm *logMessage) {
	if !strings.Contains(logm.Request.URL, "/v2/subscribe/") {
		return
	}
	var msg pubnubMessage
	if err := json.Unmarshal([]byte(logm.Response.Text), &msg); err != nil {
		log.Printf("ERROR: unmarshaling pubnub message: %v\n%v", err, logm)
		return
	}
	// log.Printf("PubnubMessage string: %s", logm.Response.Text)
	// log.Printf("PubnubMessage struct: %+v", msg)
	sec, dec := math.Modf(logm.Ts)
	ts := time.Unix(int64(sec), int64(dec*(1e9)))
	for _, m := range msg.M {
		if m.D.Channel != c.ChannelID {
			l(ts, "WARN: inconsistent channel id; got %s want %s", m.D.Channel, c.ChannelID)
		}
		if m.D.UserProfile != nil && c.UserID != 0 && m.D.UserProfile.UserID != c.UserID {
			if u, ok := c.Users[m.D.UserProfile.UserID]; ok {
				u.Profile = m.D.UserProfile
			} else {
				c.Users[m.D.UserProfile.UserID] = &User{Profile: m.D.UserProfile}
			}
			l(ts, "User update: %+v", m.D.UserProfile)
		}
		if m.D.Action == "unraise_hands" {
			if u, ok := c.Users[m.D.UserID]; ok {
				l(ts, "User unraised the hand: %+v", u.Profile)
				u.RaisedHand = false
			} else {
				l(ts, "User %d unraised the hand, but profile not found", m.D.UserID)
			}
		}
		if m.D.Action == "raise_hands" {
			l(ts, "User raised the hand: %+v", c.Users[m.D.UserProfile.UserID].Profile)
			c.Users[m.D.UserProfile.UserID].RaisedHand = true
		}
		if m.D.Action == "add_speaker" && m.D.UserProfile.UserID != c.UserID {
			l(ts, "Speaker added: %+v", c.Users[m.D.UserProfile.UserID].Profile)
			c.Users[m.D.UserProfile.UserID].RaisedHand = false
		}
		if m.D.Action == "remove_speaker" {
			if u, ok := c.Users[m.D.UserID]; ok {
				l(ts, "Speaker removed: %+v", u.Profile)
				u.Profile.IsSpeaker = false
			} else {
				l(ts, "Speaker removal for user %d, but profile not found", m.D.UserID)
			}
		}
		if m.D.Action == "leave_channel" && m.D.UserID == c.UserID {
			l(ts, "Cleaning up channel information %s", c.ChannelID)
			c.ChannelID = ""
			c.Users = make(map[int64]*User)
		} else if m.D.Action == "leave_channel" {
			if u, ok := c.Users[m.D.UserID]; ok {
				l(ts, "User left the channel: %+v", u.Profile)
				delete(c.Users, m.D.UserID)
			} else {
				l(ts, "User left the channel: %d (no profile)", m.D.UserID)
			}
		}
	}
}
