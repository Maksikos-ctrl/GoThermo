package main

import (
	"fmt"
	"regexp"
)

// mentionsUser is true when the text addresses the user with @username. The
// rule is the same as the one the app uses to highlight a mention: the @ must
// start a word, and the name must end the word or be followed by . , ! ?
func mentionsUser(text, username string) bool {
	re := mentionRegexp(username)
	return re != nil && re.MatchString(text)
}

func mentionRegexp(username string) *regexp.Regexp {
	if username == "" {
		return nil
	}
	re, err := regexp.Compile(`(?:^|\s)@` + regexp.QuoteMeta(username) + `(?:\s|$|[.,!?])`)
	if err != nil {
		return nil
	}
	return re
}

// GetMentionCounts returns, per channel, how many messages since the person
// last read it mention them. Channels without mentions are left out.
func (a *App) GetMentionCounts(token, username string) (map[string]int, error) {
	if err := requireUser(token, username); err != nil {
		return nil, err
	}
	publicChannels, err := listVisibleChannels(username)
	if err != nil {
		return nil, err
	}
	dmChannels, err := listDMChannels(username)
	if err != nil {
		return nil, err
	}

	re := mentionRegexp(username)
	if re == nil {
		return nil, fmt.Errorf("bad username")
	}

	out := map[string]int{}
	for _, ch := range append(publicChannels, dmChannels...) {
		lastRead, found, err := GetLastRead(username, ch.Name)
		if err != nil || !found {
			continue // never opened: GetUnreadCounts sets the starting point
		}
		msgs, err := GetMessages(ch.Name, 200)
		if err != nil {
			continue
		}
		n := 0
		for _, m := range msgs {
			if m.User != username && !m.IsCall && m.Timestamp.After(lastRead) && re.MatchString(m.Text) {
				n++
			}
		}
		if n > 0 {
			out[ch.Name] = n
		}
	}
	return out, nil
}
