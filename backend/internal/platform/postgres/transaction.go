package postgres

import (
	"context"
	"errors"

	"gorm.io/gorm"
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
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
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
