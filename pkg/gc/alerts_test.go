package gc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertsConfiguration(t *testing.T) {
	// 重置单例以确保测试隔离
	resetSingletonForTesting()
	defer resetSingletonForTesting()

	t.Run("默认配置应该启用告警", func(t *testing.T) {
		config := DefaultConfig()
		assert.True(t, config.EnableAlerts, "默认配置应该启用告警")
		assert.True(t, config.IsAlertsEnabled(), "IsAlertsEnabled应该返回true")
	})

	t.Run("禁用告警配置", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableAlerts = false

		assert.False(t, config.EnableAlerts, "告警应该被禁用")
		assert.False(t, config.IsAlertsEnabled(), "IsAlertsEnabled应该返回false")
	})

	t.Run("GC管理器禁用时告警也应该被禁用", func(t *testing.T) {
		config := DefaultConfig()
		config.Enabled = false
		config.EnableAlerts = true

		assert.False(t, config.IsAlertsEnabled(), "GC管理器禁用时告警也应该被禁用")
	})

	t.Run("监控器应该检查告警配置", func(t *testing.T) {
		// 创建禁用告警的配置
		config := DefaultConfig()
		config.EnableAlerts = false

		// 创建监控器并设置配置
		monitor := NewPerformanceMonitor()
		monitor.SetConfig(config)

		// 验证配置已设置
		assert.NotNil(t, monitor.config, "监控器配置应该被设置")
		assert.False(t, monitor.config.IsAlertsEnabled(), "监控器配置应该禁用告警")

		// 模拟一个会触发告警的GC执行
		// 这里我们无法直接测试告警是否被跳过，但可以确保配置正确设置
		monitor.RecordGCExecution(200*time.Millisecond, 100*1024*1024, 50*1024*1024)
	})

	t.Run("从应用配置创建服务时应该包含告警配置", func(t *testing.T) {
		// 创建模拟的应用配置
		appConfig := &mockAppGCConfig{
			enabled:          true,
			interval:         5 * time.Minute,
			memoryThreshold:  100 * 1024 * 1024,
			strategy:         "standard",
			forceGC:          false,
			freeOSMemory:     false,
			enableStats:      true,
			enableMonitoring: true,
			enableAutoTuning: false,
			enableAlerts:     true, // 启用告警
			maxRetries:       3,
			retryInterval:    10 * time.Second, // 调整为10秒，避免超时冲突
			executionTimeout: 60 * time.Second,
		}

		service, err := NewServiceFromAppGCConfig(appConfig)
		require.NoError(t, err, "创建服务不应该出错")
		require.NotNil(t, service, "服务不应该为nil")

		config := service.GetConfig()
		require.NotNil(t, config, "配置不应该为nil")
		assert.True(t, config.EnableAlerts, "告警应该被启用")
		assert.True(t, config.IsAlertsEnabled(), "IsAlertsEnabled应该返回true")
	})

	t.Run("配置重载应该更新监控器的告警设置", func(t *testing.T) {
		// 创建初始配置（启用告警）
		initialConfig := DefaultConfig()
		initialConfig.EnableAlerts = true
		initialConfig.RetryInterval = 10 * time.Second // 调整重试间隔避免超时冲突

		manager, err := NewManager(initialConfig)
		require.NoError(t, err, "创建管理器不应该出错")
		require.NotNil(t, manager, "管理器不应该为nil")

		// 验证监控器配置
		if manager.monitor != nil {
			assert.True(t, manager.monitor.config.IsAlertsEnabled(), "初始配置应该启用告警")
		}

		// 创建新配置（禁用告警）
		newConfig := DefaultConfig()
		newConfig.EnableAlerts = false
		newConfig.RetryInterval = 10 * time.Second // 调整重试间隔避免超时冲突

		// 重载配置
		err = manager.ReloadConfig(newConfig)
		require.NoError(t, err, "重载配置不应该出错")

		// 验证监控器配置已更新
		if manager.monitor != nil {
			assert.False(t, manager.monitor.config.IsAlertsEnabled(), "重载后配置应该禁用告警")
		}
	})
}

// mockAppGCConfig 模拟应用GC配置
type mockAppGCConfig struct {
	strategy         string
	maxRetries       int
	interval         time.Duration
	memoryThreshold  uint64
	executionTimeout time.Duration
	retryInterval    time.Duration
	enableStats      bool
	enableMonitoring bool
	enableAutoTuning bool
	enableAlerts     bool
	enabled          bool
	freeOSMemory     bool
	forceGC          bool
}

func (m *mockAppGCConfig) IsEnabled() bool                    { return m.enabled }
func (m *mockAppGCConfig) GetInterval() time.Duration         { return m.interval }
func (m *mockAppGCConfig) GetMemoryThreshold() uint64         { return m.memoryThreshold }
func (m *mockAppGCConfig) GetStrategy() string                { return m.strategy }
func (m *mockAppGCConfig) ShouldForceGC() bool                { return m.forceGC }
func (m *mockAppGCConfig) ShouldFreeOSMemory() bool           { return m.freeOSMemory }
func (m *mockAppGCConfig) IsStatsEnabled() bool               { return m.enableStats }
func (m *mockAppGCConfig) IsMonitoringEnabled() bool          { return m.enableMonitoring }
func (m *mockAppGCConfig) IsAutoTuningEnabled() bool          { return m.enableAutoTuning }
func (m *mockAppGCConfig) IsAlertsEnabled() bool              { return m.enableAlerts }
func (m *mockAppGCConfig) GetMaxRetries() int                 { return m.maxRetries }
func (m *mockAppGCConfig) GetRetryInterval() time.Duration    { return m.retryInterval }
func (m *mockAppGCConfig) GetExecutionTimeout() time.Duration { return m.executionTimeout }
func (m *mockAppGCConfig) GetPressureThresholds() any         { return nil }
