package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// AppConfig holds all application configuration.
type AppConfig struct {
	Web     WebConfig     `mapstructure:"web"`
	Storage StorageConfig `mapstructure:"storage"`
	GC      GCConfig      `mapstructure:"gc"`
	Log     LogConfig     `mapstructure:"log"`
	Forward ForwardConfig `mapstructure:"forward"`
	Pool    PoolConfig    `mapstructure:"pool"`
}

// WebConfig holds web server configuration.
type WebConfig struct {
	Host     string `mapstructure:"host"`
	Username string `mapstructure:"username"` // basic auth; empty = disabled
	Password string `mapstructure:"password"`
	Port     int    `mapstructure:"port"`
}

// StorageConfig holds storage configuration.
type StorageConfig struct {
	Path string `mapstructure:"path"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Path       string `mapstructure:"path"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
}

// ForwardConfig holds forwarding tuning parameters.
type ForwardConfig struct {
	PoolSize    int `mapstructure:"pool_size"`    // goroutine pool size (0 = NumCPU*64)
	BufferSize  int `mapstructure:"buffer_size"`  // I/O buffer size in bytes
	UDPTimeout  int `mapstructure:"udp_timeout"`  // UDP session idle timeout (seconds)
	DialTimeout int `mapstructure:"dial_timeout"` // outbound dial timeout (seconds)
}

// GCConfig holds garbage collection management configuration.
type GCConfig struct {
	Strategy          string `mapstructure:"strategy"`            // standard, aggressive, gentle, adaptive
	IntervalSeconds   int    `mapstructure:"interval_seconds"`    // GC interval in seconds
	MemoryThresholdMB int    `mapstructure:"memory_threshold_mb"` // memory threshold in MB (0 = disabled)
	Enabled           bool   `mapstructure:"enabled"`             // enable periodic GC
	EnableMonitoring  bool   `mapstructure:"enable_monitoring"`   // enable performance monitoring
}

// PoolConfig holds goroutine pool configuration.
type PoolConfig struct {
	Size     int  `mapstructure:"size"`      // goroutine pool capacity (0 = 10000)
	PreAlloc bool `mapstructure:"pre_alloc"` // pre-allocate goroutine pool
}

var global *AppConfig

// Load reads configuration from disk, writing defaults on first run.
func Load(configPath string) (*AppConfig, error) {
	v := viper.New()
	setDefaults(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		dir := appDataDir()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(dir)
		v.AddConfigPath(".")
	}

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			return nil, err
		}
		// First run – persist defaults so users can see the config file.
		if e2 := writeDefaults(v, configPath); e2 != nil {
			return nil, e2
		}
	}

	cfg := &AppConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	_ = os.MkdirAll(filepath.Dir(cfg.Storage.Path), 0o755)
	_ = os.MkdirAll(filepath.Dir(cfg.Log.Path), 0o755)

	global = cfg
	return cfg, nil
}

// Get returns the global AppConfig (Load must be called first).
func Get() *AppConfig { return global }

func setDefaults(v *viper.Viper) {
	dir := appDataDir()
	v.SetDefault("web.host", "127.0.0.1")
	v.SetDefault("web.port", 8080)
	v.SetDefault("storage.path", filepath.Join(dir, "data", "rules.db"))
	v.SetDefault("log.level", "info")
	v.SetDefault("log.path", filepath.Join(dir, "logs", "app.log"))
	v.SetDefault("log.max_size_mb", 50)
	v.SetDefault("log.max_backups", 5)
	v.SetDefault("log.max_age_days", 30)
	v.SetDefault("log.compress", true)
	v.SetDefault("forward.pool_size", 0)
	v.SetDefault("forward.buffer_size", 32768)
	v.SetDefault("forward.udp_timeout", 30)
	v.SetDefault("forward.dial_timeout", 10)

	// GC defaults
	v.SetDefault("gc.enabled", true)
	v.SetDefault("gc.interval_seconds", 300) // 5 minutes
	v.SetDefault("gc.strategy", "standard")
	v.SetDefault("gc.memory_threshold_mb", 100)
	v.SetDefault("gc.enable_monitoring", true)

	// Pool defaults
	v.SetDefault("pool.size", 10000)
	v.SetDefault("pool.pre_alloc", true)
}

func writeDefaults(v *viper.Viper, configPath string) error {
	dir := appDataDir()
	_ = os.MkdirAll(dir, 0o755)
	if configPath == "" {
		configPath = filepath.Join(dir, "config.yaml")
	}
	return v.WriteConfigAs(configPath)
}

// appDataDir returns the directory of the running executable.
func appDataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
