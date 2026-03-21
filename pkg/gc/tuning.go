// Package gc 提供GC动态调优功能 | Provides GC dynamic tuning
package gc

import (
	"context"
	"fmt"
	"go-port-forward/pkg/serializer/json"
	"sync"
	"time"

	"go-port-forward/pkg/logger"

	"go.uber.org/zap"
)

// 全局单例实例
var (
	globalDynamicTuner *DynamicTuner
	tunerOnce          sync.Once
)

// DynamicTuner GC动态调优器 | GC dynamic tuner
type DynamicTuner struct {
	lastTuning      time.Time
	monitor         *PerformanceMonitor
	managerRef      func() *Manager
	tuningRules     []TuningRule
	tuningCooldown  time.Duration
	mu              sync.RWMutex
	autoTuneEnabled bool
}

// TuningRule 调优规则 | Tuning rule
type TuningRule struct {
	Name        string `json:"name" msgpack:"name"`
	Description string `json:"description" msgpack:"description"`

	Action    TuningAction    `json:"action" msgpack:"action"`
	Condition TuningCondition `json:"condition" msgpack:"condition"`

	Priority int `json:"priority" msgpack:"priority"`

	Enabled bool `json:"enabled" msgpack:"enabled"`
}

// TuningCondition 调优条件 | Tuning condition
type TuningCondition struct {
	MetricType    string  `json:"metric_type" msgpack:"metric_type"`       // 指标类型 | Metric type
	Operator      string  `json:"operator" msgpack:"operator"`             // 运算符 | Operator (">", "<", ">=", "<=", "==")
	Threshold     float64 `json:"threshold" msgpack:"threshold"`           // 阈值 | Threshold
	WindowMinutes int     `json:"window_minutes" msgpack:"window_minutes"` // 时间窗口（分钟）| Time window (minutes)
}

// TuningAction 调优动作 | Tuning action
type TuningAction struct {
	Parameters map[string]any `json:"parameters" msgpack:"parameters"` // 动作参数 | Action parameters
	Type       string         `json:"type" msgpack:"type"`             // 动作类型 | Action type
}

// NewDynamicTuner 获取全局单例动态调优器 | Get global singleton dynamic tuner
func NewDynamicTuner(manager *Manager, monitor *PerformanceMonitor) *DynamicTuner {
	tunerOnce.Do(func() {
		logger.Info("Initializing global Dynamic Tuner singleton")

		globalDynamicTuner = &DynamicTuner{
			managerRef:      func() *Manager { return manager }, // 使用函数引用
			monitor:         monitor,
			tuningRules:     getDefaultTuningRules(),
			autoTuneEnabled: true,
			tuningCooldown:  2 * time.Minute, // 调优冷却时间
		}

		logger.Info("Global Dynamic Tuner initialized successfully")
	})

	return globalDynamicTuner
}

// GetGlobalDynamicTuner 直接获取全局动态调优器实例 | Get global dynamic tuner instance directly
func GetGlobalDynamicTuner() *DynamicTuner {
	return globalDynamicTuner
}

// resetTunerForTesting 重置调优器单例（仅用于测试）
func resetTunerForTesting() {
	globalDynamicTuner = nil
	tunerOnce = sync.Once{}
}

// getDefaultTuningRules 获取默认调优规则
func getDefaultTuningRules() []TuningRule {
	return []TuningRule{
		{
			Name: "High GC Frequency",
			Condition: TuningCondition{
				MetricType:    "gc_frequency",
				Operator:      ">",
				Threshold:     8.0, // 每分钟超过8次
				WindowMinutes: 5,
			},
			Action: TuningAction{
				Type: "change_strategy",
				Parameters: map[string]any{
					"strategy": "gentle",
				},
			},
			Priority:    1,
			Enabled:     true,
			Description: "当GC频率过高时，切换到温和策略",
		},
		{
			Name: "Low Memory Efficiency",
			Condition: TuningCondition{
				MetricType:    "memory_efficiency",
				Operator:      "<",
				Threshold:     0.05, // 效率低于5%
				WindowMinutes: 10,
			},
			Action: TuningAction{
				Type: "change_strategy",
				Parameters: map[string]any{
					"strategy": "aggressive",
				},
			},
			Priority:    2,
			Enabled:     true,
			Description: "当内存释放效率低时，切换到激进策略",
		},
		{
			Name: "Long GC Duration",
			Condition: TuningCondition{
				MetricType:    "avg_gc_duration",
				Operator:      ">",
				Threshold:     50.0, // 平均耗时超过50ms
				WindowMinutes: 5,
			},
			Action: TuningAction{
				Type: "adjust_interval",
				Parameters: map[string]any{
					"multiplier": 1.5, // 增加50%间隔
				},
			},
			Priority:    3,
			Enabled:     true,
			Description: "当GC耗时过长时，增加GC间隔",
		},
		{
			Name: "Memory Growth Rate High",
			Condition: TuningCondition{
				MetricType:    "memory_growth_rate",
				Operator:      ">",
				Threshold:     100 * 1024 * 1024, // 每小时增长超过100MB
				WindowMinutes: 30,
			},
			Action: TuningAction{
				Type: "adjust_interval",
				Parameters: map[string]any{
					"multiplier": 0.7, // 减少30%间隔
				},
			},
			Priority:    4,
			Enabled:     true,
			Description: "当内存增长率高时，减少GC间隔",
		},
	}
}

// StartAutoTuning 启动自动调优 | Start auto-tuning
func (dt *DynamicTuner) StartAutoTuning(ctx context.Context) {
	if !dt.autoTuneEnabled {
		return
	}

	ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
	defer ticker.Stop()

	logger.Info("GC auto-tuning started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("GC auto-tuning stopped")
			return
		case <-ticker.C:
			dt.performAutoTuning()
		}
	}
}

// performAutoTuning 执行自动调优
func (dt *DynamicTuner) performAutoTuning() {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	now := time.Now()

	// 检查调优冷却时间
	if now.Sub(dt.lastTuning) < dt.tuningCooldown {
		return
	}

	metrics := dt.monitor.GetMetrics()

	// 按优先级排序检查规则
	for _, rule := range dt.tuningRules {
		if !rule.Enabled {
			continue
		}

		if dt.evaluateCondition(rule.Condition, metrics) {
			if dt.executeAction(rule.Action) {
				dt.lastTuning = now
				logger.Info("Auto-tuning rule applied",
					zap.String("rule", rule.Name),
					zap.String("description", rule.Description))
				break // 只应用第一个匹配的规则
			}
		}
	}
}

// evaluateCondition 评估调优条件
func (dt *DynamicTuner) evaluateCondition(condition TuningCondition, metrics PerformanceMetrics) bool {
	var value float64

	switch condition.MetricType {
	case "gc_frequency":
		value = metrics.GCFrequency
	case "avg_gc_duration":
		value = float64(metrics.AvgGCDuration.Milliseconds())
	case "memory_efficiency":
		value = metrics.MemoryEfficiency
	case "gc_overhead":
		value = metrics.GCOverhead
	case "memory_growth_rate":
		value = metrics.MemoryGrowthRate
	default:
		return false
	}

	switch condition.Operator {
	case ">":
		return value > condition.Threshold
	case "<":
		return value < condition.Threshold
	case ">=":
		return value >= condition.Threshold
	case "<=":
		return value <= condition.Threshold
	case "==":
		return value == condition.Threshold
	default:
		return false
	}
}

// executeAction 执行调优动作
func (dt *DynamicTuner) executeAction(action TuningAction) bool {
	switch action.Type {
	case "change_strategy":
		return dt.changeStrategy(action.Parameters)
	case "adjust_interval":
		return dt.adjustInterval(action.Parameters)
	case "update_threshold":
		return dt.updateThreshold(action.Parameters)
	default:
		logger.Warn("Unknown tuning action type", zap.String("type", action.Type))
		return false
	}
}

// changeStrategy 更改GC策略
func (dt *DynamicTuner) changeStrategy(params map[string]any) bool {
	strategyStr, ok := params["strategy"].(string)
	if !ok {
		return false
	}

	strategy, err := createStrategy(StrategyType(strategyStr))
	if err != nil {
		logger.Error("Failed to create strategy", zap.Error(err))
		return false
	}

	// 通过函数引用获取管理器
	manager := dt.managerRef()
	if manager == nil {
		logger.Error("Manager reference is nil")
		return false
	}

	// 更新管理器的策略
	manager.mu.Lock()
	manager.strategy = strategy
	manager.mu.Unlock()

	logger.Info("GC strategy changed", zap.String("new_strategy", strategyStr))
	return true
}

// adjustInterval 调整GC间隔
func (dt *DynamicTuner) adjustInterval(params map[string]any) bool {
	multiplier, ok := params["multiplier"].(float64)
	if !ok {
		return false
	}

	// 通过函数引用获取管理器
	manager := dt.managerRef()
	if manager == nil {
		logger.Error("Manager reference is nil")
		return false
	}

	manager.mu.Lock()
	oldInterval := manager.config.Interval
	newInterval := time.Duration(float64(oldInterval) * multiplier)

	// 限制间隔范围
	if newInterval < 30*time.Second {
		newInterval = 30 * time.Second
	} else if newInterval > 30*time.Minute {
		newInterval = 30 * time.Minute
	}

	manager.config.Interval = newInterval

	// 如果管理器正在运行，需要重新创建ticker
	if manager.IsRunning() {
		manager.tickerMu.Lock()
		if manager.ticker != nil {
			manager.ticker.Stop()
			manager.ticker = time.NewTicker(newInterval)
		}
		manager.tickerMu.Unlock()
	}

	manager.mu.Unlock()

	logger.Info("GC interval adjusted",
		zap.Duration("old_interval", oldInterval),
		zap.Duration("new_interval", newInterval),
		zap.Float64("multiplier", multiplier))

	return true
}

// updateThreshold 更新内存阈值
func (dt *DynamicTuner) updateThreshold(params map[string]any) bool {
	threshold, ok := params["threshold"].(float64)
	if !ok {
		return false
	}

	// 通过函数引用获取管理器
	manager := dt.managerRef()
	if manager == nil {
		logger.Error("Manager reference is nil")
		return false
	}

	manager.mu.Lock()
	oldThreshold := manager.config.MemoryThreshold
	manager.config.MemoryThreshold = uint64(threshold)
	manager.mu.Unlock()

	logger.Info("Memory threshold updated",
		zap.Uint64("old_threshold", oldThreshold),
		zap.Uint64("new_threshold", uint64(threshold)))

	return true
}

// AddTuningRule 添加调优规则 | Add tuning rule
func (dt *DynamicTuner) AddTuningRule(rule TuningRule) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	dt.tuningRules = append(dt.tuningRules, rule)

	logger.Info("Tuning rule added",
		zap.String("name", rule.Name),
		zap.String("description", rule.Description))
}

// RemoveTuningRule 移除调优规则 | Remove tuning rule
func (dt *DynamicTuner) RemoveTuningRule(name string) bool {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	for i, rule := range dt.tuningRules {
		if rule.Name == name {
			dt.tuningRules = append(dt.tuningRules[:i], dt.tuningRules[i+1:]...)
			logger.Info("Tuning rule removed", zap.String("name", name))
			return true
		}
	}

	return false
}

// EnableAutoTuning 启用自动调优 | Enable auto-tuning
func (dt *DynamicTuner) EnableAutoTuning() {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	dt.autoTuneEnabled = true
	logger.Info("Auto-tuning enabled")
}

// DisableAutoTuning 禁用自动调优 | Disable auto-tuning
func (dt *DynamicTuner) DisableAutoTuning() {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	dt.autoTuneEnabled = false
	logger.Info("Auto-tuning disabled")
}

// GetTuningRules 获取调优规则 | Get tuning rules
func (dt *DynamicTuner) GetTuningRules() []TuningRule {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	rules := make([]TuningRule, len(dt.tuningRules))
	copy(rules, dt.tuningRules)
	return rules
}

// ExportConfiguration 导出调优配置 | Export tuning configuration
func (dt *DynamicTuner) ExportConfiguration() (string, error) {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	config := map[string]any{
		"auto_tune_enabled": dt.autoTuneEnabled,
		"tuning_cooldown":   dt.tuningCooldown,
		"tuning_rules":      dt.tuningRules,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal configuration: %w", err)
	}

	return string(data), nil
}

// ImportConfiguration 导入调优配置 | Import tuning configuration
func (dt *DynamicTuner) ImportConfiguration(configJSON string) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if autoTune, ok := config["auto_tune_enabled"].(bool); ok {
		dt.autoTuneEnabled = autoTune
	}

	if cooldown, ok := config["tuning_cooldown"].(string); ok {
		if duration, err := time.ParseDuration(cooldown); err == nil {
			dt.tuningCooldown = duration
		}
	}

	if rules, ok := config["tuning_rules"].([]any); ok {
		var newRules []TuningRule
		for _, ruleData := range rules {
			if ruleBytes, err := json.Marshal(ruleData); err == nil {
				var rule TuningRule
				if json.Unmarshal(ruleBytes, &rule) == nil {
					newRules = append(newRules, rule)
				}
			}
		}
		dt.tuningRules = newRules
	}

	logger.Info("Tuning configuration imported")
	return nil
}
