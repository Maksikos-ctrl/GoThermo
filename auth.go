package main

import (
	"fmt"
	"log"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword хеширует пароль с использованием bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash проверяет пароль с хешем
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidateEmail проверяет формат email
func ValidateEmail(email string) error {
	if !strings.Contains(email, "@") {
		return fmt.Errorf("неверный формат email")
	}
	if len(email) < 5 {
		return fmt.Errorf("email слишком короткий")
	}
	return nil
}

// ValidatePassword проверяет требования к паролю
func ValidatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("пароль должен содержать минимум 6 символов")
	}
	return nil
}

// Register регистрирует нового пользователя
func (a *App) Register(email, password string) (User, error) {
	log.Printf("📝 Попытка регистрации: %s", email)

	// Валидация
	if err := ValidateEmail(email); err != nil {
		return User{}, err
	}

	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}

	username := strings.Split(email, "@")[0]

	// Проверяем, существует ли пользователь
	if _, err := GetUserFromRedis(email); err == nil {
		log.Printf("❌ Пользователь уже существует: %s", email)
		return User{}, fmt.Errorf("пользователь с таким email уже существует")
	}

	// Хешируем пароль
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("ошибка хеширования пароля: %v", err)
	}
	log.Printf("🔐 Пароль захеширован для: %s", email)

	// Создаём пользователя
	user := userManager.RegisterUser(username, email)

	// ✅ ВАЖНО: Сначала сохраняем пользователя
	if err := SaveUserToRedis(user); err != nil {
		log.Printf("❌ Ошибка сохранения пользователя в Redis: %v", err)
		return User{}, fmt.Errorf("ошибка сохранения пользователя: %v", err)
	}
	log.Printf("💾 Пользователь сохранен в Redis: %s", email)

	// ✅ ВАЖНО: Затем сохраняем пароль
	if err := SaveUserPasswordToRedis(email, hashedPassword); err != nil {
		log.Printf("❌ Ошибка сохранения пароля в Redis: %v", err)
		return User{}, fmt.Errorf("ошибка сохранения пароля: %v", err)
	}
	log.Printf("🔐 Пароль сохранен в Redis для: %s", email)

	// Проверяем что всё сохранилось
	savedUser, _ := GetUserFromRedis(email)
	if savedUser != nil {
		log.Printf("✅ Проверка: пользователь найден в Redis: %s", savedUser.Username)
	}

	savedPass, _ := GetUserPasswordFromRedis(email)
	if savedPass != "" {
		log.Printf("✅ Проверка: пароль найден в Redis для: %s", email)
	}

	log.Printf("✅ Пользователь зарегистрирован: %s (ID: %s)", username, user.ID)

	// Broadcast статуса "online"
	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, "online")
	}

	return *user, nil
}

// Login выполняет вход с проверкой пароля
func (a *App) Login(email, password string) (User, error) {
	log.Printf("🔐 Попытка входа: %s", email)

	// Валидация
	if err := ValidateEmail(email); err != nil {
		return User{}, err
	}

	if password == "" {
		return User{}, fmt.Errorf("введите пароль")
	}

	// 1. Проверяем пользователя
	userFromRedis, err := GetUserFromRedis(email)
	if err != nil {
		log.Printf("❌ Пользователь не найден в Redis: %s", email)
		return User{}, fmt.Errorf("пользователь не найден")
	}
	log.Printf("✅ Найден пользователь: %s", userFromRedis.Username)

	// 2. Проверяем пароль
	savedHash, err := GetUserPasswordFromRedis(email)
	if err != nil {
		log.Printf("❌ Пароль не найден для: %s", email)
		// Попробуем создать пароль заново для этого пользователя (временно)
		log.Printf("🔄 Восстанавливаем пароль для: %s", email)
		hashedPassword, _ := HashPassword(password)
		SaveUserPasswordToRedis(email, hashedPassword)
		savedHash, _ = GetUserPasswordFromRedis(email)
	}

	// 3. Проверяем пароль
	if !CheckPasswordHash(password, savedHash) {
		log.Printf("❌ Неверный пароль для: %s", email)
		return User{}, fmt.Errorf("неверный пароль")
	}

	username := strings.Split(email, "@")[0]
	user := userManager.RegisterUser(username, email)

	log.Printf("✅ Пользователь вошёл: %s", username)

	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(username, "online")
	}

	return *user, nil
}
