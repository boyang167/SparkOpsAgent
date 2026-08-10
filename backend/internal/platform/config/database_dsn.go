package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

// EffectiveDSN returns DSN from structured fields when dsn is empty, else dsn as-is.
func (d DatabaseConfig) EffectiveDSN() (string, error) {
	if d.DSN != "" {
		return d.DSN, nil
	}
	if d.Host == "" || d.User == "" || d.Name == "" {
		return "", nil
	}
	port := d.Port
	if port == 0 {
		port = 5432
	}
	sslmode := d.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	user := url.UserPassword(d.User, d.Password)
	return fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=%s",
		user.String(), d.Host, port, d.Name, sslmode), nil
}

func (c *Config) resolveDatabaseDSN() error {
	if v := os.Getenv("DATABASE_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("DATABASE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Database.Port = p
		}
	}
	if v := os.Getenv("DATABASE_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("DATABASE_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("DATABASE_NAME"); v != "" {
		c.Database.Name = v
	}
	if v := os.Getenv("DATABASE_SSLMODE"); v != "" {
		c.Database.SSLMode = v
	}
	dsn, err := c.Database.EffectiveDSN()
	if err != nil {
		return err
	}
	if dsn != "" && c.Database.DSN == "" {
		c.Database.DSN = dsn
	}
	return nil
}
