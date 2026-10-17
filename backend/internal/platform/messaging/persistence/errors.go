package persistence

import (
	"errors"

	"gorm.io/gorm"
)

var ErrLeaseLost = errors.New("messaging persistence lease lost")

func requireLeaseUpdate(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLeaseLost
	}
	return nil
}
