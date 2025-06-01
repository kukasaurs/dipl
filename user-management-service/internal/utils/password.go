package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword принимает "сырой" пароль и возвращает его bcrypt-хэш
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
