package main

import (
	"fmt"
	"log"
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

func (a *App) Register(email, password string) (User, error) {
	log.Printf("📝 Attempting registration: %s", email)

	if err := ValidateNewEmail(email); err != nil {
		return User{}, err
	}

	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}

	if _, err := GetUserFromRedis(email); err == nil {
		log.Printf("❌ User already exists: %s", email)
		return User{}, fmt.Errorf("user with such email already exists")
	}

	hashedPassword, err := HashPassword(password)
	if err != nil {
		return User{}, internalFailure("hashing password", err)
	}

	user, err := createAccount(strings.Split(email, "@")[0], email)
	if err != nil {
		return User{}, internalFailure("saving new user", err)
	}
	username := user.Username

	if err := SaveUserPasswordToRedis(email, hashedPassword); err != nil {
		return User{}, internalFailure("saving password", err)
	}

	token, err := CreateSession(username)
	if err != nil {
		return User{}, sessionFailure("creating session", err)
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

	if err := checkLoginAllowed(email); err != nil {
		log.Printf("🚫 Login blocked (too many failures): %s", email)
		return User{}, err
	}

	userFromRedis, err := GetUserFromRedis(email)
	if err != nil {
		log.Printf("❌ User not found in Redis: %s", email)
		recordLoginFailure(email)
		return User{}, fmt.Errorf("incorrect email or password")
	}

	savedHash, err := GetUserPasswordFromRedis(email)
	if err != nil || savedHash == "" {
		log.Printf("❌ No password hash stored for: %s", email)
		recordLoginFailure(email)
		return User{}, fmt.Errorf("incorrect email or password")
	}

	if !CheckPasswordHash(password, savedHash) {
		log.Printf("❌ Incorrect password for: %s", email)
		recordLoginFailure(email)
		return User{}, fmt.Errorf("incorrect email or password")
	}

	clearLoginFailures(email)

	username := userFromRedis.Username
	user := userManager.RegisterUser(username, email)

	token, err := CreateSession(username)
	if err != nil {
		return User{}, sessionFailure("creating session", err)
	}

	log.Printf("✅ User logged in: %s", username)

	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, "online")
	}

	return withToken(user, token), nil
}

// createAccount picks a free username (ivan, ivan2, ...) and saves the user.
// The name is reserved in memory and checked against the store, so a name
// taken in the database but missing from the cache is skipped, not an error.
func createAccount(base, email string) (*User, error) {
	taken := map[string]bool{}
	for attempt := 0; attempt < 20; attempt++ {
		user, created := userManager.reserveUsername(base, email, taken)
		err := SaveUserToRedis(user)
		if err == nil {
			return user, nil
		}
		if created {
			userManager.release(user)
		}
		if !isUsernameTaken(err) {
			return nil, err
		}
		taken[user.Username] = true
	}
	return nil, fmt.Errorf("could not find a free username")
}
