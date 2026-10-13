package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func ValidateEmail(email string) error {
	if !strings.Contains(email, "@") {
		return fmt.Errorf("Wrong email format: missing '@'")
	}
	if len(email) < 5 {
		return fmt.Errorf("Wrong email format: too short")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("Wrong password format: too short")
	}
	return nil
}

// uniqueUsername builds a username from a base string and appends a number
// if that username is already taken by a different account (e.g.
// ivan@gmail.com and ivan@mail.ru both want "ivan").
func uniqueUsername(base, email string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "user"
	}
	candidate := base
	for i := 2; ; i++ {
		existing, taken := userManager.GetUser(candidate)
		if !taken || (existing.Email == email && existing.Provider == "") {
			return candidate
		}
		candidate = base + strconv.Itoa(i)
	}
}

func (a *App) Register(email, password string) (User, error) {
	log.Printf("📝 Attempting registration: %s", email)

	if err := ValidateEmail(email); err != nil {
		return User{}, err
	}

	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}

	if _, err := GetUserFromRedis(email); err == nil {
		log.Printf("❌ User already exists: %s", email)
		return User{}, fmt.Errorf("user with such email already exists")
	}

	username := uniqueUsername(strings.Split(email, "@")[0], email)

	hashedPassword, err := HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("error hashing password: %v", err)
	}

	user := userManager.RegisterUser(username, email)

	if err := SaveUserToRedis(user); err != nil {
		log.Printf("❌ Error saving user to Redis: %v", err)
		return User{}, fmt.Errorf("error saving user: %v", err)
	}

	if err := SaveUserPasswordToRedis(email, hashedPassword); err != nil {
		log.Printf("❌ Error saving password to Redis: %v", err)
		return User{}, fmt.Errorf("error saving password: %v", err)
	}

	token, err := CreateSession(username)
	if err != nil {
		return User{}, fmt.Errorf("error creating session: %v", err)
	}

	log.Printf("✅ User registered: %s (ID: %s)", username, user.ID)

	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, "online")
	}

	return withToken(user, token), nil
}

func (a *App) Login(email, password string) (User, error) {
	log.Printf("🔐 Attempting login: %s", email)

	if err := ValidateEmail(email); err != nil {
		return User{}, err
	}

	if password == "" {
		return User{}, fmt.Errorf("please enter a password")
	}

	userFromRedis, err := GetUserFromRedis(email)
	if err != nil {
		log.Printf("❌ User not found in Redis: %s", email)
		return User{}, fmt.Errorf("user not found")
	}

	// No password hash stored: this account cannot be logged into with a
	// password (never silently save whatever password was just typed).
	savedHash, err := GetUserPasswordFromRedis(email)
	if err != nil || savedHash == "" {
		log.Printf("❌ No password hash stored for: %s", email)
		return User{}, fmt.Errorf("incorrect email or password")
	}

	if !CheckPasswordHash(password, savedHash) {
		log.Printf("❌ Incorrect password for: %s", email)
		return User{}, fmt.Errorf("incorrect password")
	}

	// Use the username stored with the account (it may differ from the email
	// prefix now that usernames are made unique).
	username := userFromRedis.Username
	user := userManager.RegisterUser(username, email)

	token, err := CreateSession(username)
	if err != nil {
		return User{}, fmt.Errorf("error creating session: %v", err)
	}

	log.Printf("✅ User logged in: %s", username)

	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, "online")
	}

	return withToken(user, token), nil
}
