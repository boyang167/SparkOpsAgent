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
	if IsProduction() {
		if c.JWT.Secret == "dev-only-change-me" || c.JWT.Secret == "docker-dev-secret" {
			return fmt.Errorf("jwt.secret must be changed in production")
		}
		if c.Messaging.Driver == "inmemory" {
			return fmt.Errorf("messaging.driver must not be inmemory in production")
		}
		if c.Delivery.Driver == "mock" {
			return fmt.Errorf("delivery.driver must not be mock in production")
		}
		if c.Runtime.Driver == "mock" {
			return fmt.Errorf("runtime.driver must not be mock in production")
		}
		if c.Messaging.Driver == "amqp" && c.Messaging.AMQP.URL == "" {
			return fmt.Errorf("messaging.amqp.url is required when driver=amqp")
		}
	}
	return nil
}
