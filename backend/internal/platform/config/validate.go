package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

func AppEnv() string {
	if v := strings.TrimSpace(os.Getenv("APP_ENV")); v != "" {
		return strings.ToLower(v)
	}
	return EnvDevelopment
}

func IsProduction() bool {
	return AppEnv() == EnvProduction
}

// Validate checks config before serving traffic.
func (c *Config) Validate() error {
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if len(c.JWT.Secret) < 16 {
		return fmt.Errorf("jwt.secret must be at least 16 characters")
	}
	if c.Deploy.JenkinsTimeout <= 0 {
		return fmt.Errorf("deploy.jenkins_timeout must be greater than zero")
	}
	if c.Deploy.RolloutTimeout <= 0 {
		return fmt.Errorf("deploy.rollout_timeout must be greater than zero")
	}
	if c.Deploy.ReconcileInterval <= 0 {
		return fmt.Errorf("deploy.reconcile_interval must be greater than zero")
	}
	switch c.Delivery.Driver {
	case "mock":
		// Explicit mock driver for local demos only.
	case "jenkins":
		// Jenkins URL/credentials come from each project's jenkins_secret_id.
	default:
		return fmt.Errorf("unsupported delivery.driver: %s", c.Delivery.Driver)
	}
	switch c.Runtime.Driver {
	case "mock", "k8s":
	default:
		return fmt.Errorf("unsupported runtime.driver: %s", c.Runtime.Driver)
	}
	messagingDriver := strings.ToLower(strings.TrimSpace(c.Messaging.Driver))
	if messagingDriver == "" {
		messagingDriver = "inmemory"
	}
	switch messagingDriver {
	case "inmemory":
	case "amqp":
		if strings.TrimSpace(c.Messaging.AMQP.URL) == "" {
			return fmt.Errorf("messaging.amqp.url is required when driver=amqp")
		}
		if strings.TrimSpace(c.Messaging.AMQP.EventExchange) == "" {
			return fmt.Errorf("messaging.amqp.event_exchange is required when driver=amqp")
		}
		if c.Messaging.Outbox.PollInterval <= 0 {
			return fmt.Errorf("messaging.outbox.poll_interval must be greater than zero")
		}
		if c.Messaging.Outbox.BatchSize <= 0 {
			return fmt.Errorf("messaging.outbox.batch_size must be greater than zero")
		}
		if c.Messaging.Outbox.LockTimeout <= 0 {
			return fmt.Errorf("messaging.outbox.lock_timeout must be greater than zero")
		}
	default:
		return fmt.Errorf("unsupported messaging.driver: %s", c.Messaging.Driver)
	}
	if IsProduction() {
		if c.JWT.Secret == "dev-only-change-me" || c.JWT.Secret == "docker-dev-secret" {
			return fmt.Errorf("jwt.secret must be changed in production")
		}
		if messagingDriver == "inmemory" {
			return fmt.Errorf("messaging.driver must not be inmemory in production")
		}
		if c.Delivery.Driver == "mock" {
			return fmt.Errorf("delivery.driver must not be mock in production")
		}
		if c.Runtime.Driver == "mock" {
			return fmt.Errorf("runtime.driver must not be mock in production")
		}
	}
	return nil
}
