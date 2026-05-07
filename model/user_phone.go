package model

import (
	"strings"
)

// GetUserByPhone returns user by normalized phone (E.164, e.g. +8613800000000).
func GetUserByPhone(phone string) (*User, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil, nil
	}
	var user User
	if err := DB.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func IsPhoneAlreadyTaken(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false
	}
	var count int64
	DB.Model(&User{}).Where("phone = ?", phone).Count(&count)
	return count > 0
}

