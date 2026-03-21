package gc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultConfigValidation 测试默认配置的有效性
// Test default configuration validation
func TestDefaultConfigValidation(t *testing.T) {
	config := DefaultConfig()
	err := config.Validate()
	require.NoError(t, err, "默认配置应该是有效的 | Default config should be valid")
}

// TestRetryConfigurationValidation 测试重试配置验证
// Test retry configuration validation
func TestRetryConfigurationValidation(t *testing.T) {
	tests := []struct {
		name             string
		description      string
		maxRetries       int
		retryInterval    time.Duration
		executionTimeout time.Duration
		shouldPass       bool
	}{
		{
			name:             "有效配置 - 默认值",
			maxRetries:       2,
			retryInterval:    10 * time.Second,
			executionTimeout: 60 * time.Second,
			shouldPass:       true,
			description:      "2 × 10s = 20s < 60s × 70% = 42s",
		},
		{
			name:             "有效配置 - 边界值",
			maxRetries:       3,
			retryInterval:    10 * time.Second,
			executionTimeout: 60 * time.Second,
			shouldPass:       true,
			description:      "3 × 10s = 30s < 60s × 70% = 42s",
		},
		{
			name:             "无效配置 - 重试间隔过长",
			maxRetries:       3,
			retryInterval:    30 * time.Second,
			executionTimeout: 60 * time.Second,
			shouldPass:       false,
			description:      "3 × 30s = 90s > 60s × 70% = 42s",
		},
		{
			name:             "无效配置 - 单次重试间隔过长",
			maxRetries:       1,
			retryInterval:    40 * time.Second,
			executionTimeout: 60 * time.Second,
			shouldPass:       false,
			description:      "40s > 60s / 2 = 30s",
		},
		{
			name:             "有效配置 - 较短的重试间隔",
			maxRetries:       5,
			retryInterval:    5 * time.Second,
			executionTimeout: 60 * time.Second,
			shouldPass:       true,
			description:      "5 × 5s = 25s < 60s × 70% = 42s",
		},
		{
			name:             "有效配置 - 较长的执行超时",
			maxRetries:       3,
			retryInterval:    30 * time.Second,
			executionTimeout: 180 * time.Second,
			shouldPass:       true,
			description:      "3 × 30s = 90s < 180s × 70% = 126s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Enabled:          true,
				Interval:         5 * time.Minute,
				MemoryThreshold:  100 * 1024 * 1024,
				Strategy:         StrategyStandard,
				MaxRetries:       tt.maxRetries,
				RetryInterval:    tt.retryInterval,
				ExecutionTimeout: tt.executionTimeout,
			}

			err := config.Validate()
			if tt.shouldPass {
				assert.NoError(t, err, "配置应该有效: %s", tt.description)
			} else {
				assert.Error(t, err, "配置应该无效: %s", tt.description)
			}
		})
	}
}

// TestConfigFromYAML 测试从YAML配置创建服务
// Test creating service from YAML config
func TestConfigFromYAML(t *testing.T) {
	// 重置单例以确保测试隔离
	resetSingletonForTesting()
	defer resetSingletonForTesting()

	// 模拟从配置文件加载的配置
	yamlConfig := map[string]any{
		"enabled":            true,
		"interval":           300,
		"memory_threshold":   104857600,
		"strategy":           "standard",
		"force_gc":           false,
		"free_os_memory":     false,
		"enable_stats":       true,
		"enable_monitoring":  true,
		"enable_auto_tuning": false,
		"enable_alerts":      true,
		"max_retries":        2,
		"retry_interval":     10,
		"execution_timeout":  60,
	}

	service, err := NewServiceFromAppConfig(yamlConfig)
	require.NoError(t, err, "从YAML配置创建服务应该成功")
	require.NotNil(t, service, "服务不应该为nil")

	config := service.GetConfig()
	require.NotNil(t, config, "配置不应该为nil")

	// 验证配置值
	assert.Equal(t, 2, config.MaxRetries, "MaxRetries应该为2")
	assert.Equal(t, 10*time.Second, config.RetryInterval, "RetryInterval应该为10秒")
	assert.Equal(t, 60*time.Second, config.ExecutionTimeout, "ExecutionTimeout应该为60秒")

	// 验证配置有效性
	err = config.Validate()
	assert.NoError(t, err, "配置应该是有效的")
}

// TestRetryConfigurationEdgeCases 测试重试配置的边界情况
// Test retry configuration edge cases
func TestRetryConfigurationEdgeCases(t *testing.T) {
	t.Run("零重试次数", func(t *testing.T) {
		config := DefaultConfig()
		config.MaxRetries = 0
		err := config.Validate()
		assert.NoError(t, err, "零重试次数应该是有效的")
	})

	t.Run("负重试次数", func(t *testing.T) {
		config := DefaultConfig()
		config.MaxRetries = -1
		err := config.Validate()
		assert.Error(t, err, "负重试次数应该是无效的")
	})

	t.Run("零重试间隔", func(t *testing.T) {
		config := DefaultConfig()
		config.RetryInterval = 0
		err := config.Validate()
		assert.Error(t, err, "零重试间隔应该是无效的")
	})

	t.Run("零执行超时", func(t *testing.T) {
		config := DefaultConfig()
		config.ExecutionTimeout = 0
		err := config.Validate()
		assert.Error(t, err, "零执行超时应该是无效的")
	})
}
