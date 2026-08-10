package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Addr string
}

type Client struct {
	raw *goredis.Client
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return &Client{}, nil
	}
	raw := goredis.NewClient(&goredis.Options{Addr: cfg.Addr})
	return &Client{raw: raw}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c.raw == nil {
		return nil
	}
	return c.raw.Ping(ctx).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if c.raw == nil {
		return "", fmt.Errorf("redis not configured")
	}
	return c.raw.Get(ctx, key).Result()
}

func (c *Client) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if c.raw == nil {
		return fmt.Errorf("redis not configured")
	}
	return c.raw.Set(ctx, key, val, ttl).Err()
}

func (c *Client) Close() error {
	if c.raw == nil {
		return nil
	}
	return c.raw.Close()
}
