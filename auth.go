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

	
	if err := ValidateEmail(email); err != nil {
		return User{}, err
	}

	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}

	username := strings.Split(email, "@")[0]

	
	if _, err := GetUserFromRedis(email); err == nil {
		log.Printf("❌ User already exists: %s", email)
		return User{}, fmt.Errorf("user with such email already exists")
	}

	
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("error hashing password: %v", err)
	}
	log.Printf("🔐 Password hashed for: %s", email)

	
	user := userManager.RegisterUser(username, email)

	
	if err := SaveUserToRedis(user); err != nil {
		log.Printf("❌ Error saving user to Redis: %v", err)
		return User{}, fmt.Errorf("error saving user: %v", err)
	}
	log.Printf("💾 User saved to Redis: %s", email)

	
	if err := SaveUserPasswordToRedis(email, hashedPassword); err != nil {
		log.Printf("❌ Error saving password to Redis: %v", err)
		return User{}, fmt.Errorf("error saving password: %v", err)
	}
	log.Printf("🔐 Password saved to Redis for: %s", email)

	
	savedUser, _ := GetUserFromRedis(email)
	if savedUser != nil {
		log.Printf("✅ Check: user found in Redis: %s", savedUser.Username)
	}

	savedPass, _ := GetUserPasswordFromRedis(email)
	if savedPass != "" {
		log.Printf("✅ Check: password found in Redis for: %s", email)
	}

	log.Printf("✅ User registered: %s (ID: %s)", username, user.ID)

	
	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, "online")
	}

	return *user, nil
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
	log.Printf("✅ Found user: %s", userFromRedis.Username)

	
	savedHash, err := GetUserPasswordFromRedis(email)
	if err != nil {
		log.Printf("❌ Password not found for: %s", email)
		
		log.Printf("🔄 Restoring password for: %s", email)
		hashedPassword, _ := HashPassword(password)
		SaveUserPasswordToRedis(email, hashedPassword)
		savedHash, _ = GetUserPasswordFromRedis(email)
	}

	
	if !CheckPasswordHash(password, savedHash) {
		log.Printf("❌ Incorrect password for: %s", email)
		return User{}, fmt.Errorf("incorrect password")
	}

	username := strings.Split(email, "@")[0]
	user := userManager.RegisterUser(username, email)

	log.Printf("✅ User logged in: %s", username)

	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, "online")
	}

	return *user, nil
}
