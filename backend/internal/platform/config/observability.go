package config

type ObservabilityConfig struct {
	Log LogConfig `yaml:"log"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	Directory  string `yaml:"directory"`
	Console    bool   `yaml:"console"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

func (c *LogConfig) WithDefaults() LogConfig {
	isZero := c.Level == "" && c.Directory == "" && c.MaxSizeMB == 0 && c.MaxBackups == 0 && !c.Console
	out := *c
	if out.Level == "" {
		out.Level = "info"
	}
	if out.Directory == "" {
		out.Directory = "logs"
	}
	if out.MaxSizeMB <= 0 {
		out.MaxSizeMB = 100
	}
	if out.MaxBackups <= 0 {
		out.MaxBackups = 7
	}
	if isZero {
		out.Console = true
	}
	return out
}
