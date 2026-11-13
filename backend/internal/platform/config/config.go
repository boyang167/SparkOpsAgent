package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	Messaging     MessagingConfig     `yaml:"messaging"`
	K8s           K8sConfig           `yaml:"k8s"`
	Runtime       RuntimeConfig       `yaml:"runtime"`
	Delivery      DeliveryConfig      `yaml:"delivery"`
	Deploy        DeployConfig        `yaml:"deploy"`
	Jenkins       JenkinsConfig       `yaml:"jenkins"`
	GitLab        GitLabConfig        `yaml:"gitlab"`
	Webhook       WebhookConfig       `yaml:"webhook"`
	JWT           JWTConfig           `yaml:"jwt"`
	Security      SecurityConfig      `yaml:"security"`
	Auth          AuthConfig          `yaml:"auth"`
	Policy        PolicyConfig        `yaml:"policy"`
	LLM           LLMConfig           `yaml:"llm"`
	Observability ObservabilityConfig `yaml:"observability"`
	Notify        NotifyConfig        `yaml:"notify"`
	Vault         VaultConfig         `yaml:"vault"`
	Filesystem    FilesystemConfig    `yaml:"filesystem"`
}

type FilesystemConfig struct {
	TransferGateway TransferGatewayConfig      `yaml:"transfer_gateway"`
	Reconciler      FilesystemReconcilerConfig `yaml:"reconciler"`
	SFTP            SFTPGatewayConfig          `yaml:"sftp"`
}

type SFTPGatewayConfig struct {
	Enabled          bool          `yaml:"enabled"`
	Listen           string        `yaml:"listen"`
	HostKeySecretRef string        `yaml:"host_key_secret_ref"`
	LandingZone      string        `yaml:"landing_zone"`
	IdleTimeout      time.Duration `yaml:"idle_timeout"`
}

type FilesystemReconcilerConfig struct {
	Interval      time.Duration `yaml:"interval"`
	LeaseTTL      time.Duration `yaml:"lease_ttl"`
	BatchSize     int           `yaml:"batch_size"`
	GCGracePeriod time.Duration `yaml:"gc_grace_period"`
}

type TransferGatewayConfig struct {
	Listen             string `yaml:"listen"`
	PublicURL          string `yaml:"public_url"`
	CapabilityAudience string `yaml:"capability_audience"`
	CapabilitySecret   string `yaml:"capability_secret"`
	MaxFileBytes       int64  `yaml:"max_file_bytes"`
	StorageRoot        string `yaml:"storage_root"`
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
	Addr      string `yaml:"addr"`
	PublicURL string `yaml:"public_url"`
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
	Driver string       `yaml:"driver"`
	AMQP   AMQPConfig   `yaml:"amqp"`
	Outbox OutboxConfig `yaml:"outbox"`
}

type AMQPConfig struct {
	URL           string `yaml:"url"`
	Exchange      string `yaml:"exchange"`
	ExchangeType  string `yaml:"exchange_type"`
	EventExchange string `yaml:"event_exchange"`
}

type OutboxConfig struct {
	PollInterval   time.Duration `yaml:"poll_interval"`
	BatchSize      int           `yaml:"batch_size"`
	LockTimeout    time.Duration `yaml:"lock_timeout"`
	PublishTimeout time.Duration `yaml:"publish_timeout"`
}

type K8sConfig struct {
	DefaultCluster string             `yaml:"default_cluster"`
	KubeconfigPath string             `yaml:"kubeconfig_path"`
	Clusters       []K8sClusterConfig `yaml:"clusters"`
}

type K8sClusterConfig struct {
	Name    string `yaml:"name"`
	Context string `yaml:"context"`
}

type RuntimeConfig struct {
	Driver  string `yaml:"driver"`
	Cluster string `yaml:"cluster"`
}

type DeliveryConfig struct {
	Driver string `yaml:"driver"`
	URL    string `yaml:"url"`
	User   string `yaml:"user"`
	Token  string `yaml:"token"`
}

type DeployConfig struct {
	JenkinsTimeout    time.Duration `yaml:"jenkins_timeout"`
	RolloutTimeout    time.Duration `yaml:"rollout_timeout"`
	ReconcileInterval time.Duration `yaml:"reconcile_interval"`
}

type JenkinsConfig struct {
	URL       string `yaml:"url"`
	User      string `yaml:"user"`
	Token     string `yaml:"token"`
	JobSuffix string `yaml:"job_suffix"`
}

type GitLabConfig struct {
	BaseURL     string `yaml:"base_url"`
	Token       string `yaml:"token"`
	ProjectPath string `yaml:"project_path_prefix"` // optional group prefix, e.g. "platform"
}

type WebhookConfig struct {
	Secret string `yaml:"secret"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	ExpireHour int    `yaml:"expire_hour"`
}

type SecurityConfig struct {
	SecretEncryptionKey string `yaml:"secret_encryption_key"`
}

type NotifyConfig struct {
	SMTP   SMTPConfig   `yaml:"smtp"`
	Feishu FeishuConfig `yaml:"feishu"`
}

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

type FeishuConfig struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

type VaultConfig struct {
	Address string `yaml:"address"`
	Token   string `yaml:"token"`
}

func (c *Config) PublicBaseURL() string {
	if c == nil {
		return "http://127.0.0.1:8080"
	}
	if base := strings.TrimRight(strings.TrimSpace(c.Server.PublicURL), "/"); base != "" {
		return base
	}
	addr := strings.TrimSpace(c.Server.Addr)
	if addr == "" {
		return "http://127.0.0.1:8080"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/")
	}
	return "http://" + addr
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
	if err := cfg.applyEnvOverrides(); err != nil {
		return nil, fmt.Errorf("environment override: %w", err)
	}
	cfg.resolveDrivers()
	if err := cfg.resolveDatabaseDSN(); err != nil {
		return nil, fmt.Errorf("database dsn: %w", err)
	}
	return &cfg, nil
}

func (c *Config) applyEnvOverrides() error {
	if v := os.Getenv("SPARKOPS_PUBLIC_URL"); v != "" {
		c.Server.PublicURL = v
	}
	if v := os.Getenv("VAULT_ADDR"); v != "" {
		c.Vault.Address = v
	}
	if v := os.Getenv("VAULT_TOKEN"); v != "" {
		c.Vault.Token = v
	}
	if v := os.Getenv("SMTP_HOST"); v != "" {
		c.Notify.SMTP.Host = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Notify.SMTP.Port = port
		}
	}
	if v := os.Getenv("SMTP_USERNAME"); v != "" {
		c.Notify.SMTP.Username = v
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		c.Notify.SMTP.Password = v
	}
	if v := os.Getenv("SMTP_FROM"); v != "" {
		c.Notify.SMTP.From = v
	}
	if v := os.Getenv("FEISHU_APP_ID"); v != "" {
		c.Notify.Feishu.AppID = v
	}
	if v := os.Getenv("FEISHU_APP_SECRET"); v != "" {
		c.Notify.Feishu.AppSecret = v
	}
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
	if v := os.Getenv("AMQP_EVENT_EXCHANGE"); v != "" {
		c.Messaging.AMQP.EventExchange = v
	}
	if v := os.Getenv("OUTBOX_POLL_INTERVAL"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("OUTBOX_POLL_INTERVAL: %w", err)
		}
		c.Messaging.Outbox.PollInterval = duration
	}
	if v := os.Getenv("OUTBOX_BATCH_SIZE"); v != "" {
		batchSize, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("OUTBOX_BATCH_SIZE: %w", err)
		}
		c.Messaging.Outbox.BatchSize = batchSize
	}
	if v := os.Getenv("OUTBOX_LOCK_TIMEOUT"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("OUTBOX_LOCK_TIMEOUT: %w", err)
		}
		c.Messaging.Outbox.LockTimeout = duration
	}
	if v := os.Getenv("OUTBOX_PUBLISH_TIMEOUT"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("OUTBOX_PUBLISH_TIMEOUT: %w", err)
		}
		c.Messaging.Outbox.PublishTimeout = duration
	}
	if v := os.Getenv("KUBECONFIG"); v != "" {
		c.K8s.KubeconfigPath = v
	}
	if v := os.Getenv("RUNTIME_DRIVER"); v != "" {
		c.Runtime.Driver = v
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
	if v := os.Getenv("DEPLOY_JENKINS_TIMEOUT"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("DEPLOY_JENKINS_TIMEOUT: %w", err)
		}
		c.Deploy.JenkinsTimeout = duration
	}
	if v := os.Getenv("DEPLOY_ROLLOUT_TIMEOUT"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("DEPLOY_ROLLOUT_TIMEOUT: %w", err)
		}
		c.Deploy.RolloutTimeout = duration
	}
	if v := os.Getenv("DEPLOY_RECONCILE_INTERVAL"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("DEPLOY_RECONCILE_INTERVAL: %w", err)
		}
		c.Deploy.ReconcileInterval = duration
	}
	if v := os.Getenv("FILESYSTEM_RECONCILER_INTERVAL"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("FILESYSTEM_RECONCILER_INTERVAL: %w", err)
		}
		c.Filesystem.Reconciler.Interval = duration
	}
	if v := os.Getenv("FILESYSTEM_RECONCILER_LEASE_TTL"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("FILESYSTEM_RECONCILER_LEASE_TTL: %w", err)
		}
		c.Filesystem.Reconciler.LeaseTTL = duration
	}
	if v := os.Getenv("FILESYSTEM_RECONCILER_BATCH_SIZE"); v != "" {
		batchSize, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("FILESYSTEM_RECONCILER_BATCH_SIZE: %w", err)
		}
		c.Filesystem.Reconciler.BatchSize = batchSize
	}
	if v := os.Getenv("FILESYSTEM_RECONCILER_GC_GRACE_PERIOD"); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("FILESYSTEM_RECONCILER_GC_GRACE_PERIOD: %w", err)
		}
		c.Filesystem.Reconciler.GCGracePeriod = duration
	}
	return nil
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
	if c.Messaging.AMQP.EventExchange == "" {
		c.Messaging.AMQP.EventExchange = "sparkops.events"
	}
	if c.Messaging.Outbox.PollInterval == 0 {
		c.Messaging.Outbox.PollInterval = time.Second
	}
	if c.Messaging.Outbox.BatchSize == 0 {
		c.Messaging.Outbox.BatchSize = 100
	}
	if c.Messaging.Outbox.LockTimeout == 0 {
		c.Messaging.Outbox.LockTimeout = time.Minute
	}
	if c.Messaging.Outbox.PublishTimeout == 0 {
		c.Messaging.Outbox.PublishTimeout = 30 * time.Second
	}
	if c.Runtime.Driver == "" {
		c.Runtime.Driver = "k8s"
	}
	if c.Delivery.Driver == "" {
		c.Delivery.Driver = "jenkins"
	}
	if c.Deploy.JenkinsTimeout == 0 {
		c.Deploy.JenkinsTimeout = 30 * time.Minute
	}
	if c.Deploy.RolloutTimeout == 0 {
		c.Deploy.RolloutTimeout = 10 * time.Minute
	}
	if c.Deploy.ReconcileInterval == 0 {
		c.Deploy.ReconcileInterval = 30 * time.Second
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
	if c.Filesystem.TransferGateway.Listen == "" {
		c.Filesystem.TransferGateway.Listen = ":8082"
	}
	if c.Filesystem.TransferGateway.CapabilityAudience == "" {
		c.Filesystem.TransferGateway.CapabilityAudience = "sparkops-file-transfer"
	}
	if c.Filesystem.TransferGateway.MaxFileBytes == 0 {
		c.Filesystem.TransferGateway.MaxFileBytes = 21474836480
	}
	if c.Filesystem.TransferGateway.CapabilitySecret == "" {
		c.Filesystem.TransferGateway.CapabilitySecret = c.JWT.Secret
	}
	if c.Filesystem.TransferGateway.StorageRoot == "" {
		c.Filesystem.TransferGateway.StorageRoot = "./data/filesystem"
	}
	if c.Filesystem.Reconciler.Interval == 0 {
		c.Filesystem.Reconciler.Interval = 30 * time.Second
	}
	if c.Filesystem.Reconciler.LeaseTTL == 0 {
		c.Filesystem.Reconciler.LeaseTTL = 90 * time.Second
	}
	if c.Filesystem.Reconciler.BatchSize == 0 {
		c.Filesystem.Reconciler.BatchSize = 100
	}
	if c.Filesystem.Reconciler.GCGracePeriod == 0 {
		c.Filesystem.Reconciler.GCGracePeriod = 168 * time.Hour
	}
	if c.Filesystem.SFTP.Listen == "" {
		c.Filesystem.SFTP.Listen = ":2022"
	}
	if c.Filesystem.SFTP.LandingZone == "" {
		c.Filesystem.SFTP.LandingZone = "./data/sftp-landing"
	}
	if c.Filesystem.SFTP.IdleTimeout == 0 {
		c.Filesystem.SFTP.IdleTimeout = 15 * time.Minute
	}
}

// resolveDrivers normalizes driver names after defaults and env overrides.
func (c *Config) resolveDrivers() {
	switch strings.ToLower(strings.TrimSpace(c.Runtime.Driver)) {
	case "", "auto", "k8s":
		c.Runtime.Driver = "k8s"
	case "mock":
		c.Runtime.Driver = "mock"
	}
	switch strings.ToLower(strings.TrimSpace(c.Delivery.Driver)) {
	case "", "auto", "jenkins":
		c.Delivery.Driver = "jenkins"
	case "mock":
		c.Delivery.Driver = "mock"
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
