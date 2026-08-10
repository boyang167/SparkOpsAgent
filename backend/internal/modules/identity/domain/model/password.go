package model

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 12

type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) bool
}

type BcryptHasher struct{}

func (BcryptHasher) Hash(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	return string(bytes), err
}

func (BcryptHasher) Compare(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
