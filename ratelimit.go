package main

import (
	"fmt"
	"strings"
	"time"
)

// Brute-force protection for password login.
//
// Failed attempts are counted per email in Redis (loginfail:<email>) with a
// TTL. After maxLoginFailures failures in a row the account is locked for
// loginLockWindow; a successful login resets the counter. Counting by email
// (not by IP) fits a desktop app, where every request comes from 127.0.0.1.
const (
	maxLoginFailures = 5
	loginLockWindow  = 15 * time.Minute
)

func loginFailKey(email string) string {
	return "loginfail:" + strings.ToLower(strings.TrimSpace(email))
}

func checkLoginAllowed(email string) error {
	key := loginFailKey(email)

	n, err := redisClient.Get(ctx, key).Int()
	if err != nil || n < maxLoginFailures {
		return nil
	}

	mins := int(loginLockWindow.Minutes())
	if ttl, err := redisClient.TTL(ctx, key).Result(); err == nil && ttl > 0 {
		mins = int(ttl.Minutes()) + 1
	}
	return fmt.Errorf("too many failed attempts, try again in %d min", mins)
}

func recordLoginFailure(email string) {
	key := loginFailKey(email)

	n, err := redisClient.Incr(ctx, key).Result()
	if err != nil {
		return
	}
	if n == 1 || n >= maxLoginFailures {
		redisClient.Expire(ctx, key, loginLockWindow)
	}
}

func clearLoginFailures(email string) {
	redisClient.Del(ctx, loginFailKey(email))
}
