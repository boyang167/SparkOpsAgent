package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrCommitOutcomeUnknown = errors.New("postgres transaction commit outcome unknown")
)

type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

type txKey struct{}

type gormTransactor struct {
	db *gorm.DB
}

func NewTransactor(db *gorm.DB) Transactor {
	return &gormTransactor{db: db}
}

func (t *gormTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	tx := t.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	txCtx := context.WithValue(ctx, txKey{}, tx)
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback().Error
			panic(recovered)
		}
	}()
	if err := fn(txCtx); err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("%w: %v", ErrCommitOutcomeUnknown, err)
	}
	return nil
}

type directTransactor struct{}

func NewDirectTransactor() Transactor {
	return directTransactor{}
}

func (directTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func DB(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}

func RequireTx(ctx context.Context) (*gorm.DB, error) {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	if !ok {
		return nil, errors.New("postgres transaction required")
	}
	return tx.WithContext(ctx), nil
}
