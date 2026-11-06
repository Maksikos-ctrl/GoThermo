package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// The admin account.
//
// It is not stored with the other users: its name and password come from the
// server's environment (GOTHERMO_ADMIN_USER, default "admin", and
// GOTHERMO_ADMIN_PASSWORD). Without a password there is no admin at all. The
// name is reserved, so nobody can register as it, and every admin method
// checks the session on the server; the app showing the panel is only a view.

func adminName() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("GOTHERMO_ADMIN_USER")))
	if v == "" {
		v = "admin"
	}
	return v
}

func adminPassword() string { return os.Getenv("GOTHERMO_ADMIN_PASSWORD") }

func isAdminName(username string) bool {
	return adminPassword() != "" && strings.EqualFold(strings.TrimSpace(username), adminName())
}

// adminLogin signs the admin in. identifier is what was typed in the login
// box; anything without an "@" is tried as the admin's name.
func adminLogin(identifier, password string) (User, error) {
	bad := fmt.Errorf("incorrect email or password")
	key := "admin:" + strings.ToLower(strings.TrimSpace(identifier))
	if len(key) > 64 {
		key = key[:64]
	}
	if err := checkLoginAllowed(key); err != nil {
		return User{}, err
	}
	want := adminPassword()
	a := sha256.Sum256([]byte(password))
	b := sha256.Sum256([]byte(want))
	passOK := subtle.ConstantTimeCompare(a[:], b[:]) == 1
	if want == "" || !strings.EqualFold(strings.TrimSpace(identifier), adminName()) || !passOK {
		recordLoginFailure(key)
		return User{}, bad
	}
	clearLoginFailures(key)
	token, err := CreateSession(adminName())
	if err != nil {
		return User{}, sessionFailure("creating admin session", err)
	}
	log.Printf("🛡️ admin signed in")
	return User{ID: "admin", Username: adminName(), Status: "offline", Token: token, IsAdmin: true}, nil
}

// requireAdmin returns nil only for a valid session of the admin account.
func requireAdmin(token string) error {
	u, err := sessionUser(token)
	if err != nil || !isAdminName(u) {
		return errNotAuthorized
	}
	return nil
}

// IsAdmin tells the app which screen to show after sign-in.
func (a *App) IsAdmin(token string) bool { return requireAdmin(token) == nil }

type AdminStats struct {
	Users   int `json:"users"`
	Online  int `json:"online"`
	Banned  int `json:"banned"`
	Reports int `json:"reports"`
}

type AdminUser struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Provider  string `json:"provider,omitempty"`
	Status    string `json:"status"`
	LastSeen  string `json:"lastSeen,omitempty"`
	HasAvatar bool   `json:"hasAvatar"`
	Banned    bool   `json:"banned"`
}

func (a *App) AdminOverview(token string) (AdminStats, error) {
	if err := requireAdmin(token); err != nil {
		return AdminStats{}, err
	}
	var s AdminStats
	for _, u := range userManager.GetAllUsers() {
		s.Users++
		if u.Status == "online" || u.Status == "away" {
			s.Online++
		}
	}
	if n, err := redisClient.SCard(ctx, bannedKey).Result(); err == nil {
		s.Banned = int(n)
	}
	if n, err := redisClient.LLen(ctx, reportsKey).Result(); err == nil {
		s.Reports = int(n)
	}
	return s, nil
}

func (a *App) AdminUsers(token string) ([]AdminUser, error) {
	if err := requireAdmin(token); err != nil {
		return nil, err
	}
	banned, _ := redisClient.SMembers(ctx, bannedKey).Result()
	isBan := make(map[string]bool, len(banned))
	for _, b := range banned {
		isBan[b] = true
	}
	avatars, _ := redisClient.HGetAll(ctx, avatarsKey).Result()
	seen := map[string]bool{}
	out := []AdminUser{}
	for _, u := range userManager.GetAllUsers() {
		k := strings.ToLower(u.Username)
		seen[k] = true
		out = append(out, AdminUser{
			Username: u.Username, Email: u.Email, Provider: u.Provider, Status: u.Status,
			LastSeen: u.LastSeen, HasAvatar: avatars[k] != "", Banned: isBan[k],
		})
	}
	for _, b := range banned { // banned names the user list no longer knows
		if !seen[b] {
			out = append(out, AdminUser{Username: b, Status: "offline", Banned: true})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username)
	})
	return out, nil
}

func (a *App) AdminReports(token string) ([]Report, error) {
	if err := requireAdmin(token); err != nil {
		return nil, err
	}
	raw, err := redisClient.LRange(ctx, reportsKey, 0, -1).Result()
	if err != nil {
		return nil, internalFailure("admin reports", err)
	}
	out := make([]Report, 0, len(raw))
	for _, s := range raw {
		var r Report
		if json.Unmarshal([]byte(s), &r) == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// findReport returns a report and its stored form (needed to remove it).
func findReport(id string) (Report, string, bool) {
	raw, err := redisClient.LRange(ctx, reportsKey, 0, -1).Result()
	if err != nil {
		return Report{}, "", false
	}
	for _, s := range raw {
		var r Report
		if json.Unmarshal([]byte(s), &r) == nil && r.ID == id {
			return r, s, true
		}
	}
	return Report{}, "", false
}

func (a *App) AdminDismissReport(token, reportID string) error {
	if err := requireAdmin(token); err != nil {
		return err
	}
	_, raw, ok := findReport(reportID)
	if !ok {
		return nil
	}
	return redisClient.LRem(ctx, reportsKey, 1, raw).Err()
}

// AdminDeleteReportedMessage removes the message a report points at, whoever
// wrote it, and closes the report.
func (a *App) AdminDeleteReportedMessage(token, reportID string) error {
	if err := requireAdmin(token); err != nil {
		return err
	}
	r, raw, ok := findReport(reportID)
	if !ok {
		return fmt.Errorf("that report is already closed")
	}
	msgs, err := GetMessages(r.Channel, 1000)
	if err == nil {
		for _, m := range msgs {
			if m.ID != r.MessageID {
				continue
			}
			if err := DeleteMessageInRedis(r.Channel, r.MessageID); err != nil {
				return internalFailure("admin delete", err)
			}
			deleteThread(r.Channel, r.MessageID)
			if removed, _ := removePin(r.Channel, r.MessageID); removed {
				a.announcePins(r.Channel)
			}
			if m.IsFile {
				removeUpload(m.FileID)
			}
			if a.hub != nil {
				a.hub.BroadcastMessageDeleted(r.Channel, r.MessageID)
			}
			break
		}
	}
	log.Printf("🛡️ admin removed message %s in #%s", r.MessageID, r.Channel)
	return redisClient.LRem(ctx, reportsKey, 1, raw).Err()
}

func (a *App) AdminBan(token, username string) error {
	if err := requireAdmin(token); err != nil {
		return err
	}
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return fmt.Errorf("no username")
	}
	if isAdminName(name) {
		return fmt.Errorf("the admin account cannot be banned")
	}
	if err := redisClient.SAdd(ctx, bannedKey, name).Err(); err != nil {
		return internalFailure("admin ban", err)
	}
	log.Printf("🛡️ admin banned %s", name)
	return nil
}

func (a *App) AdminUnban(token, username string) error {
	if err := requireAdmin(token); err != nil {
		return err
	}
	name := strings.ToLower(strings.TrimSpace(username))
	if err := redisClient.SRem(ctx, bannedKey, name).Err(); err != nil {
		return internalFailure("admin unban", err)
	}
	log.Printf("🛡️ admin unbanned %s", name)
	return nil
}
