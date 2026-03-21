// Package gc 提供垃圾回收管理功能 | Provides garbage collection management
package gc

import (
	"errors"
	"time"
)

// Config GC配置结构体 | GC configuration struct
type Config struct {

	// PressureThresholds 内存压力阈值配置 | Memory pressure threshold configuration
	PressureThresholds *PressureThresholds `json:"pressure_thresholds,omitempty" msgpack:"pressure_thresholds,omitempty" yaml:"pressure_thresholds,omitempty" mapstructure:"pressure_thresholds,omitempty"`

	// Strategy GC策略类型 | GC strategy type
	Strategy StrategyType `json:"strategy" msgpack:"strategy" yaml:"strategy" mapstructure:"strategy"`

	// Interval 定时GC间隔时间 | Scheduled GC interval
	Interval time.Duration `json:"interval" msgpack:"interval" yaml:"interval" mapstructure:"interval"`

	// RetryInterval 重试间隔时间 | Retry interval
	RetryInterval time.Duration `json:"retry_interval" msgpack:"retry_interval" yaml:"retry_interval" mapstructure:"retry_interval"`

	// MaxRetries 最大重试次数（当GC失败时）| Max retries (when GC fails)
	MaxRetries int `json:"max_retries" msgpack:"max_retries" yaml:"max_retries" mapstructure:"max_retries"`

	// ExecutionTimeout GC执行超时时间 | GC execution timeout
	ExecutionTimeout time.Duration `json:"execution_timeout" msgpack:"execution_timeout" yaml:"execution_timeout" mapstructure:"execution_timeout"`

	// MemoryThreshold 内存阈值触发GC（字节数，0表示禁用）| Memory threshold to trigger GC (bytes, 0 to disable)
	MemoryThreshold uint64 `json:"memory_threshold" msgpack:"memory_threshold" yaml:"memory_threshold" mapstructure:"memory_threshold"`

	// Enabled 是否启用GC管理器 | Whether to enable GC manager
	Enabled bool `json:"enabled" msgpack:"enabled" yaml:"enabled" mapstructure:"enabled"`

	// ForceGC 是否强制执行完整GC | Whether to force full GC
	ForceGC bool `json:"force_gc" msgpack:"force_gc" yaml:"force_gc" mapstructure:"force_gc"`

	// FreeOSMemory 是否释放操作系统内存 | Whether to free OS memory
	FreeOSMemory bool `json:"free_os_memory" msgpack:"free_os_memory" yaml:"free_os_memory" mapstructure:"free_os_memory"`

	// EnableStats 是否启用统计信息收集 | Whether to enable statistics collection
	EnableStats bool `json:"enable_stats" msgpack:"enable_stats" yaml:"enable_stats" mapstructure:"enable_stats"`

	// EnableMonitoring 是否启用性能监控 | Whether to enable performance monitoring
	EnableMonitoring bool `json:"enable_monitoring" msgpack:"enable_monitoring" yaml:"enable_monitoring" mapstructure:"enable_monitoring"`

	// EnableAutoTuning 是否启用自动调优 | Whether to enable auto tuning
	EnableAutoTuning bool `json:"enable_auto_tuning" msgpack:"enable_auto_tuning" yaml:"enable_auto_tuning" mapstructure:"enable_auto_tuning"`

	// EnableAlerts 是否启用GC告警 | Whether to enable GC alerts
	EnableAlerts bool `json:"enable_alerts" msgpack:"enable_alerts" yaml:"enable_alerts" mapstructure:"enable_alerts"`
}

// PressureThresholds 内存压力阈值配置 | Memory pressure threshold configuration
type PressureThresholds struct {
	// Low 低压力阈值 | Low pressure threshold
	Low uint64 `json:"low" msgpack:"low" yaml:"low" mapstructure:"low"`
	// Medium 中等压力阈值 | Medium pressure threshold
	Medium uint64 `json:"medium" msgpack:"medium" yaml:"medium" mapstructure:"medium"`
	// High 高压力阈值 | High pressure threshold
	High uint64 `json:"high" msgpack:"high" yaml:"high" mapstructure:"high"`
	// Critical 临界压力阈值 | Critical pressure threshold
	Critical uint64 `json:"critical" msgpack:"critical" yaml:"critical" mapstructure:"critical"`
}

// StrategyType GC策略类型 | GC strategy type
type StrategyType string

const (
	// StrategyStandard 标准GC策略（runtime.GC）| Standard GC strategy (runtime.GC)
	StrategyStandard StrategyType = "standard"

	// StrategyAggressive 激进GC策略（runtime.GC + debug.FreeOSMemory）| Aggressive GC strategy
	StrategyAggressive StrategyType = "aggressive"

	// StrategyGentle 温和GC策略（仅在内存压力大时执行）| Gentle GC strategy (only when memory pressure is high)
	StrategyGentle StrategyType = "gentle"

	// StrategyAdaptive 自适应GC策略（根据内存增长速度调整）| Adaptive GC strategy (adjusts based on memory growth rate)
	StrategyAdaptive StrategyType = "adaptive"

	// StrategyPressureAware 内存压力感知GC策略 | Memory pressure-aware GC strategy
	StrategyPressureAware StrategyType = "pressure_aware"

	// StrategyScheduled 基于时间调度的GC策略 | Time-scheduled GC strategy
	StrategyScheduled StrategyType = "scheduled"

	// StrategyCustom 自定义GC策略 | Custom GC strategy
	StrategyCustom StrategyType = "custom"
)

// DefaultConfig 返回默认GC配置 | Return default GC configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		Interval:         5 * time.Minute,   // 默认5分钟执行一次GC
		MemoryThreshold:  100 * 1024 * 1024, // 默认100MB内存阈值
		Strategy:         StrategyStandard,
		ForceGC:          false,
		FreeOSMemory:     false,
		EnableStats:      true,
		MaxRetries:       2,
		RetryInterval:    10 * time.Second,
		ExecutionTimeout: 60 * time.Second, // 默认60秒执行超时
		EnableMonitoring: true,
		EnableAutoTuning: false, // 默认关闭自动调优
		EnableAlerts:     true,  // 默认启用告警
		PressureThresholds: &PressureThresholds{
			Low:      50 * 1024 * 1024,  // 50MB
			Medium:   100 * 1024 * 1024, // 100MB
			High:     200 * 1024 * 1024, // 200MB
			Critical: 500 * 1024 * 1024, // 500MB
		},
	}
}

// Validate 验证配置有效性 | Validate configuration
func (c *Config) Validate() error {
	if c == nil {
		return ErrInvalidConfig
	}

	if c.Enabled {
		if c.Interval <= 0 {
			return ErrInvalidInterval
		}

		if c.MaxRetries < 0 {
			return ErrInvalidRetries
		}

		if c.RetryInterval <= 0 {
			return ErrInvalidRetryInterval
		}

		if c.ExecutionTimeout <= 0 {
			return errors.New("execution timeout must be greater than 0")
		}

		// 验证 retry_interval 和 execution_timeout 的配置合理性
		if err := c.validateRetryConfiguration(); err != nil {
			return err
		}

		// 验证策略类型
		switch c.Strategy {
		case StrategyStandard, StrategyAggressive, StrategyGentle,
			StrategyAdaptive, StrategyPressureAware, StrategyScheduled, StrategyCustom:
			// 有效策略
		default:
			return ErrInvalidStrategy
		}
	}

	return nil
}

// validateRetryConfiguration 验证重试配置的合理性
func (c *Config) validateRetryConfiguration() error {
	if c.MaxRetries <= 0 || c.RetryInterval <= 0 || c.ExecutionTimeout <= 0 {
		return nil // 基础验证已经在上面完成
	}

	// 检查单次重试间隔是否过长
	if c.RetryInterval > c.ExecutionTimeout/2 {
		return errors.New("retry_interval is too large compared to execution_timeout. " +
			"retry_interval should be less than or equal to half of execution_timeout")
	}

	// 计算理论上的最小重试时间（仅重试间隔，不包括实际执行时间）
	minRetryTime := time.Duration(c.MaxRetries) * c.RetryInterval

	// 如果仅重试间隔就超过执行超时时间的70%，认为配置不合理
	if minRetryTime > time.Duration(float64(c.ExecutionTimeout)*0.7) {
		return errors.New("potential timeout conflict: retry intervals alone consume too much of execution_timeout. " +
			"Consider: 1) increasing execution_timeout, 2) reducing retry_interval, or 3) reducing max_retries")
	}

	// 更严格的检查：如果重试间隔 × 重试次数 > 执行超时时间的一半
	if minRetryTime > c.ExecutionTimeout/2 {
		return errors.New("retry configuration may cause timeout: (max_retries × retry_interval) exceeds half of execution_timeout")
	}

	return nil
}

// GetRecommendedConfiguration 获取推荐的配置 | Get recommended configuration
func (c *Config) GetRecommendedConfiguration() map[string]any {
	recommendations := make(map[string]any)

	// 基于策略推荐超时时间
	switch c.Strategy {
	case StrategyGentle:
		recommendations["execution_timeout"] = "90s-120s"
		recommendations["retry_interval"] = "10s-15s"
		recommendations["max_retries"] = "3-5"
	case StrategyStandard:
		recommendations["execution_timeout"] = "60s-90s"
		recommendations["retry_interval"] = "5s-10s"
		recommendations["max_retries"] = "3-4"
	case StrategyAggressive:
		recommendations["execution_timeout"] = "30s-60s"
		recommendations["retry_interval"] = "3s-5s"
		recommendations["max_retries"] = "2-3"
	case StrategyAdaptive:
		recommendations["execution_timeout"] = "120s-180s"
		recommendations["retry_interval"] = "15s-20s"
		recommendations["max_retries"] = "3-5"
	default:
		recommendations["execution_timeout"] = "60s"
		recommendations["retry_interval"] = "10s"
		recommendations["max_retries"] = "3"
	}

	// 添加配置原则说明
	recommendations["principles"] = []string{
		"execution_timeout should be at least 3x (max_retries * retry_interval)",
		"retry_interval should be less than execution_timeout / 2",
		"For production environments, prefer longer timeouts with fewer retries",
		"For development environments, shorter timeouts with more frequent retries are acceptable",
	}

	return recommendations
}

// IsMemoryThresholdEnabled 检查是否启用了内存阈值触发 | Check if memory threshold trigger is enabled
func (c *Config) IsMemoryThresholdEnabled() bool {
	return c.Enabled && c.MemoryThreshold > 0
}

// ShouldFreeOSMemory 检查是否应该释放操作系统内存 | Check if OS memory should be freed
func (c *Config) ShouldFreeOSMemory() bool {
	return c.Enabled && c.FreeOSMemory
}

// IsStatsEnabled 检查是否启用统计信息收集 | Check if statistics collection is enabled
func (c *Config) IsStatsEnabled() bool {
	return c.Enabled && c.EnableStats
}

// IsAlertsEnabled 检查是否启用GC告警 | Check if GC alerts are enabled
func (c *Config) IsAlertsEnabled() bool {
	return c.Enabled && c.EnableAlerts
}
