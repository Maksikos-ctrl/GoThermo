package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	IsOnline bool   `json:"isOnline"`
	Status   string `json:"status"` // "online", "away", "offline"
	LastSeen string `json:"lastSeen,omitempty"`
}

type UserManager struct {
	users      map[string]*User
	userTokens map[string]string
	mu         sync.RWMutex
}

var userManager = &UserManager{
	users:      make(map[string]*User),
	userTokens: make(map[string]string),
}

func generateID() string {
	return uuid.New().String()
}

// ✅ ДОБАВЛЕНО - сброс всех статусов в offline при старте
func (um *UserManager) ResetAllStatusesToOffline() {
	um.mu.Lock()
	defer um.mu.Unlock()

	for _, user := range um.users {
		user.Status = "offline"
		user.IsOnline = false
		user.LastSeen = time.Now().Format(time.RFC3339)
		go SaveUserToRedis(user)
	}

	log.Printf("✓ Все статусы сброшены в offline (%d пользователей)", len(um.users))
}

func (um *UserManager) RegisterUser(username, email string) *User {
	um.mu.Lock()
	defer um.mu.Unlock()

	if existingUser, exists := um.users[username]; exists {
		log.Printf("🔄 Обновляем существующего пользователя: %s", username)
		existingUser.IsOnline = true
		existingUser.Status = "online"
		existingUser.LastSeen = time.Now().Format(time.RFC3339)

		go SaveUserToRedis(existingUser)
		return existingUser
	}

	user := &User{
		ID:       generateID(),
		Username: username,
		Email:    email,
		IsOnline: true,
		Status:   "online",
		LastSeen: time.Now().Format(time.RFC3339),
	}

	um.users[username] = user
	log.Printf("✅ Создан новый пользователь в памяти: %s (ID: %s)", username, user.ID)

	go SaveUserToRedis(user)

	return user
}

func (um *UserManager) GetUser(username string) (*User, bool) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[username]
	return user, exists
}

func (um *UserManager) UpdateUserStatus(username, status string) bool {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[username]
	if !exists {
		log.Printf("User not found: %s", username)
		return false
	}

	user.Status = status
	user.IsOnline = status != "offline"
	user.LastSeen = time.Now().Format(time.RFC3339)

	go SaveUserToRedis(user)

	log.Printf("User %s status updated to: %s", username, status)
	return true
}

func (um *UserManager) GetAllUsers() []User {
	um.mu.RLock()
	defer um.mu.RUnlock()

	users := make([]User, 0, len(um.users))
	for _, user := range um.users {
		users = append(users, *user)
	}
	return users
}

func (um *UserManager) LoadUsersFromRedis() {
	usersFromRedis, err := GetAllUsersFromRedis()
	if err != nil {
		log.Printf("Error loading users from Redis: %v", err)
		return
	}

	um.mu.Lock()
	defer um.mu.Unlock()

	for _, user := range usersFromRedis {
		um.users[user.Username] = &user
	}

	log.Printf("Loaded %d users from Redis", len(usersFromRedis))
}

func (um *UserManager) SetUserToken(token, username string) {
	um.mu.Lock()
	defer um.mu.Unlock()
	um.userTokens[token] = username
}

func (um *UserManager) GetUserByToken(token string) (*User, bool) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	username, exists := um.userTokens[token]
	if !exists {
		return nil, false
	}

	user, userExists := um.users[username]
	return user, userExists
}

func (um *UserManager) RemoveUserToken(token string) {
	um.mu.Lock()
	defer um.mu.Unlock()
	delete(um.userTokens, token)
}

// Wails API

func (a *App) GetUsers() []User {
	return userManager.GetAllUsers()
}

func (a *App) UpdateUserStatus(username, status string) bool {
	validStatuses := map[string]bool{
		"online":  true,
		"away":    true,
		"offline": true,
	}

	if !validStatuses[status] {
		log.Printf("Invalid status: %s", status)
		return false
	}

	success := userManager.UpdateUserStatus(username, status)

	// Broadcast через WebSocket
	if success && globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, status)
	}

	return success
}

func (a *App) CheckAuth(token string) (string, error) {
	user, exists := userManager.GetUserByToken(token)
	if !exists {
		return "", nil
	}

	userJSON, err := json.Marshal(user)
	if err != nil {
		return "", err
	}

	return string(userJSON), nil
}

func (a *App) Logout(token string) bool {
	user, exists := userManager.GetUserByToken(token)
	if exists {
		userManager.UpdateUserStatus(user.Username, "offline")

		// Broadcast через WebSocket
		if globalHub != nil {
			globalHub.BroadcastStatusUpdate(user.Username, "offline")
		}
	}
	userManager.RemoveUserToken(token)
	return true
}
