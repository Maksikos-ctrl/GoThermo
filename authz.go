package main

import "fmt"

// Every App method that the frontend can call goes through one of these
// helpers first. The session token is the only thing we trust: the username
// that arrives as an ordinary argument is just compared against it.

var errNotAuthorized = fmt.Errorf("not authorized, please sign in again")

// sessionUser returns the username that owns a valid session token.
func sessionUser(token string) (string, error) {
	username, ok := GetSessionUsername(token)
	if !ok {
		return "", errNotAuthorized
	}
	return username, nil
}

// requireUser checks the token is valid AND belongs to the claimed user.
func requireUser(token, claimed string) error {
	username, err := sessionUser(token)
	if err != nil {
		return err
	}
	if claimed != username {
		return errNotAuthorized
	}
	return nil
}

// requireChannelAccess lets a user into a private chat only if they are a
// member. Public channels (and names that do not exist yet) are open.
func requireChannelAccess(username, channel string) error {
	ch, err := GetChannel(channel)
	if err != nil || ch == nil {
		return nil
	}
	if ch.IsPrivate && !channelHasMember(ch.Members, username) {
		return fmt.Errorf("you are not a participant of this chat")
	}
	return nil
}
