package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Calls go peer to peer. Behind a phone network or a strict firewall that
// fails without a relay, so a server can hand out TURN credentials too.
// Without GOTHERMO_TURN_URLS / GOTHERMO_TURN_SECRET only STUN is offered.

const iceTTL = 12 * time.Hour

type iceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// turnCredentials follows coturn's "use-auth-secret" scheme: the username is
// "<expiry>:<user>" and the password is base64(HMAC-SHA1(secret, username)).
func turnCredentials(secret, user string, now time.Time) (string, string) {
	name := fmt.Sprintf("%d:%s", now.Add(iceTTL).Unix(), user)
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(name))
	return name, base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildIceServers(urlsEnv, secret, user string, now time.Time) []iceServer {
	servers := []iceServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}
	var urls []string
	for _, u := range strings.Split(urlsEnv, ",") {
		if u = strings.TrimSpace(u); u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 || secret == "" {
		return servers
	}
	name, cred := turnCredentials(secret, user, now)
	return append(servers, iceServer{URLs: urls, Username: name, Credential: cred})
}

// GetIceServers returns the ICE server list as JSON for the signed-in user.
func (a *App) GetIceServers(token string) (string, error) {
	username, err := sessionUser(token)
	if err != nil {
		return "", err
	}
	list := buildIceServers(os.Getenv("GOTHERMO_TURN_URLS"), os.Getenv("GOTHERMO_TURN_SECRET"), username, time.Now())
	b, err := json.Marshal(list)
	if err != nil {
		return "", internalFailure("ice servers", err)
	}
	return string(b), nil
}
