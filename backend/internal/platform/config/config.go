package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	Messaging     MessagingConfig     `yaml:"messaging"`
	K8s           K8sConfig           `yaml:"k8s"`
	Runtime       RuntimeConfig       `yaml:"runtime"`
	Consul        ConsulConfig        `yaml:"consul"`
	Delivery      DeliveryConfig      `yaml:"delivery"`
	Jenkins       JenkinsConfig       `yaml:"jenkins"`
	GitLab        GitLabConfig        `yaml:"gitlab"`
	Webhook       WebhookConfig       `yaml:"webhook"`
	JWT           JWTConfig           `yaml:"jwt"`
	Auth          AuthConfig          `yaml:"auth"`
	Policy        PolicyConfig        `yaml:"policy"`
	LLM           LLMConfig           `yaml:"llm"`
	Observability ObservabilityConfig `yaml:"observability"`
}

type PolicyConfig struct {
	Driver string    `yaml:"driver"`
	OPA    OPAConfig `yaml:"opa"`
}

type OPAConfig struct {
	URL      string `yaml:"url"`
	DataPath string `yaml:"data_path"`
}

type AuthConfig struct {
	OIDC OIDCConfig `yaml:"oidc"`
	LDAP LDAPConfig `yaml:"ldap"`
}

type OIDCConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
	Scopes       string `yaml:"scopes"`
	AuthURL      string `yaml:"auth_url"`
	TokenURL     string `yaml:"token_url"`
	UserInfoURL  string `yaml:"userinfo_url"`
}

type LDAPConfig struct {
	Enabled          bool   `yaml:"enabled"`
	URL              string `yaml:"url"`
	BindDN           string `yaml:"bind_dn"`
	BindPassword     string `yaml:"bind_password"`
	UserSearchBase   string `yaml:"user_search_base"`
	UserSearchFilter string `yaml:"user_search_filter"`
	EmailAttribute   string `yaml:"email_attribute"`
	NameAttribute    string `yaml:"name_attribute"`
}

type LLMConfig struct {
	DefaultProvider string                    `yaml:"default_provider"`
	Providers       map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	DSN      string `yaml:"dsn"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"sslmode"`
}

type RedisConfig struct {
	Addr string `yaml:"addr"`
}

type MessagingConfig struct {
	Driver string     `yaml:"driver"`
	AMQP   AMQPConfig `yaml:"amqp"`
}

type AMQPConfig struct {
	URL          string `yaml:"url"`
	Exchange     string `yaml:"exchange"`
	ExchangeType string `yaml:"exchange_type"`
}

type K8sConfig struct {
	DefaultCluster string              `yaml:"default_cluster"`
	KubeconfigPath string              `yaml:"kubeconfig_path"`
	Clusters       []K8sClusterConfig  `yaml:"clusters"`
}

type K8sClusterConfig struct {
	Name    string `yaml:"name"`
	Context string `yaml:"context"`
}

type RuntimeConfig struct {
	Driver       string `yaml:"driver"`
	Cluster      string `yaml:"cluster"`
	IstioEnabled bool   `yaml:"istio_enabled"`
}

type ConsulConfig struct {
	Driver     string `yaml:"driver"`
	Address    string `yaml:"address"`
	Token      string `yaml:"token"`
	Datacenter string `yaml:"datacenter"`
	KeyPrefix  string `yaml:"key_prefix"`
}

type DeliveryConfig struct {
	Driver string `yaml:"driver"`
	URL    string `yaml:"url"`
	User   string `yaml:"user"`
	Token  string `yaml:"token"`
}

type JenkinsConfig struct {
	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Token    string `yaml:"token"`
	JobSuffix string `yaml:"job_suffix"`
}

type GitLabConfig struct {
	BaseURL    string `yaml:"base_url"`
	Token      string `yaml:"token"`
	ProjectPath string `yaml:"project_path_prefix"` // optional group prefix, e.g. "platform"
}

type WebhookConfig struct {
	Secret string `yaml:"secret"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	ExpireHour int    `yaml:"expire_hour"`
}

func LoadFromEnv() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config.yaml"
	}
	return Load(path)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	cfg.applyEnvOverrides()
	if err := cfg.resolveDatabaseDSN(); err != nil {
		return nil, fmt.Errorf("database dsn: %w", err)
	}
	return &cfg, nil
}

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("MESSAGING_DRIVER"); v != "" {
		c.Messaging.Driver = v
	}
	if v := os.Getenv("AMQP_URL"); v != "" {
		c.Messaging.AMQP.URL = v
	}
	if v := os.Getenv("KUBECONFIG"); v != "" {
		c.K8s.KubeconfigPath = v
	}
	if v := os.Getenv("RUNTIME_DRIVER"); v != "" {
		c.Runtime.Driver = v
	}
	if v := os.Getenv("RUNTIME_ISTIO_ENABLED"); v == "true" || v == "1" {
		c.Runtime.IstioEnabled = true
	}
	if v := os.Getenv("CONSUL_DRIVER"); v != "" {
		c.Consul.Driver = v
	}
	if v := os.Getenv("CONSUL_ADDR"); v != "" {
		c.Consul.Address = v
	}
	if v := os.Getenv("CONSUL_TOKEN"); v != "" {
		c.Consul.Token = v
	}
	if v := os.Getenv("DELIVERY_DRIVER"); v != "" {
		c.Delivery.Driver = v
	}
	if v := os.Getenv("DELIVERY_JENKINS_URL"); v != "" {
		c.Delivery.URL = v
	}
	if v := os.Getenv("DELIVERY_JENKINS_USER"); v != "" {
		c.Delivery.User = v
	}
	if v := os.Getenv("DELIVERY_JENKINS_TOKEN"); v != "" {
		c.Delivery.Token = v
	}
	if v := os.Getenv("JENKINS_URL"); v != "" {
		c.Jenkins.URL = v
	}
	if v := os.Getenv("JENKINS_USER"); v != "" {
		c.Jenkins.User = v
	}
	if v := os.Getenv("JENKINS_TOKEN"); v != "" {
		c.Jenkins.Token = v
	}
	if v := os.Getenv("GITLAB_BASE_URL"); v != "" {
		c.GitLab.BaseURL = v
	}
	if v := os.Getenv("GITLAB_TOKEN"); v != "" {
		c.GitLab.Token = v
	}
	if v := os.Getenv("WEBHOOK_SECRET"); v != "" {
		c.Webhook.Secret = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		c.JWT.Secret = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		if c.LLM.Providers == nil {
			c.LLM.Providers = make(map[string]ProviderConfig)
		}
		p := c.LLM.Providers["openai"]
		p.APIKey = v
		c.LLM.Providers["openai"] = p
	}
}

func (c *Config) applyDefaults() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.JWT.Secret == "" {
		c.JWT.Secret = "dev-only-change-me"
	}
	if c.JWT.ExpireHour == 0 {
		c.JWT.ExpireHour = 24
	}
	if c.LLM.DefaultProvider == "" {
		c.LLM.DefaultProvider = "openai"
	}
	if c.Messaging.Driver == "" {
		c.Messaging.Driver = "inmemory"
	}
	if c.Messaging.AMQP.Exchange == "" {
		c.Messaging.AMQP.Exchange = "sparkops"
	}
	if c.Messaging.AMQP.ExchangeType == "" {
		c.Messaging.AMQP.ExchangeType = "direct"
	}
	if c.Runtime.Driver == "" {
		c.Runtime.Driver = "mock"
	}
	if c.Consul.Driver == "" {
		c.Consul.Driver = "mock"
	}
	if c.Consul.KeyPrefix == "" {
		c.Consul.KeyPrefix = "sparkops"
	}
	if c.Delivery.Driver == "" {
		c.Delivery.Driver = "mock"
	}
	if c.Jenkins.JobSuffix == "" {
		c.Jenkins.JobSuffix = "-build"
	}
	if c.Policy.Driver == "" {
		c.Policy.Driver = "casbin"
	}
	if c.Policy.OPA.DataPath == "" {
		c.Policy.OPA.DataPath = "sparkops/authz/allow"
	}
	if c.Auth.OIDC.Scopes == "" {
		c.Auth.OIDC.Scopes = "openid email profile"
	}
	if c.Auth.LDAP.UserSearchFilter == "" {
		c.Auth.LDAP.UserSearchFilter = "(uid=%s)"
	}
	if c.Auth.LDAP.EmailAttribute == "" {
		c.Auth.LDAP.EmailAttribute = "mail"
	}
	if c.Auth.LDAP.NameAttribute == "" {
		c.Auth.LDAP.NameAttribute = "cn"
	}
}

// JenkinsEffective returns Jenkins connection settings, preferring delivery.* over jenkins.*.
func (c *Config) JenkinsEffective() JenkinsConfig {
	j := c.Jenkins
	if c.Delivery.URL != "" {
		j.URL = c.Delivery.URL
	}
	if c.Delivery.User != "" {
		j.User = c.Delivery.User
	}
	if c.Delivery.Token != "" {
		j.Token = c.Delivery.Token
	}
	return j
}
