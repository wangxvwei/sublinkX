package models

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int
	Username string
	Password string
	Role     string
	Nickname string
}

func (user *User) Create() error { // 创建用户
	if err := user.hashPassword(); err != nil {
		return err
	}
	return DB.Create(user).Error
}
func (user *User) Set(UpdateUser *User) error { // 设置用户
	if err := UpdateUser.hashPassword(); err != nil {
		return err
	}
	return DB.Where("username = ?", user.Username).Updates(UpdateUser).Error
}
func (user *User) Verify() error { // 验证用户
	rawPassword := user.Password
	if err := DB.Where("username = ?", user.Username).First(user).Error; err != nil {
		return err
	}
	if strings.HasPrefix(user.Password, "$2a$") || strings.HasPrefix(user.Password, "$2b$") || strings.HasPrefix(user.Password, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(rawPassword))
	}
	if user.Password != rawPassword {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	user.Password = rawPassword
	return user.Set(&User{Password: rawPassword})
}

func (user *User) Find() error { // 查找用户
	return DB.Where("username = ? ", user.Username).First(user).Error
}

func (user *User) All() ([]User, error) { // 获取所有用户
	var users []User
	err := DB.Find(&users).Error
	return users, err
}

func (user *User) Del() error { // 删除用户
	return DB.Delete(user).Error
}

func (user *User) hashPassword() error {
	if user.Password == "" || strings.HasPrefix(user.Password, "$2a$") || strings.HasPrefix(user.Password, "$2b$") || strings.HasPrefix(user.Password, "$2y$") {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	return nil
}
