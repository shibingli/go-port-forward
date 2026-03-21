// Package gc 提供垃圾回收管理功能 | Provides garbage collection management
//
// 本包基于utils/retry的设计模式，提供定时循环GC功能。
// This package provides scheduled GC based on retry design patterns.
// 支持多种GC策略、内存阈值触发、统计信息收集等功能。
// Supports multiple GC strategies, memory threshold triggers, and statistics collection.
package gc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"go-port-forward/pkg/logger"
	"go-port-forward/pkg/pool"
	"go-port-forward/pkg/retry"

	"go.uber.org/zap"
)

// 全局单例实例 | Global singleton instances
var (
	globalGCManager *Manager
	globalGCService *Service
	managerOnce     sync.Once
	serviceOnce     sync.Once
	singletonMu     sync.RWMutex // 保护单例重置的互斥锁
)

// 预定义的错误 | Predefined errors
var (
	ErrInvalidConfig         = errors.New("invalid gc config")
	ErrInvalidInterval       = errors.New("gc interval must be greater than 0")
	ErrInvalidRetries        = errors.New("max retries must be non-negative")
	ErrInvalidRetryInterval  = errors.New("retry interval must be greater than 0")
	ErrInvalidStrategy       = errors.New("invalid gc strategy")
	ErrManagerStopped        = errors.New("gc manager is stopped")
	ErrManagerAlreadyRunning = errors.New("gc manager is already running")
)

// Strategy GC策略接口 | GC strategy interface
type Strategy interface {
	// Execute 执行GC操作 | Execute GC operation
	Execute(ctx context.Context) error

	// Name 返回策略名称 | Return strategy name
	Name() string
}

// HealthStatus 健康状态 | Health status
type HealthStatus struct {
	// Details 详细信息 | Detail information
	Details map[string]any `json:"details" msgpack:"details"`

	// LastCheck 最后检查时间 | Last check time
	LastCheck time.Time `json:"last_check" msgpack:"last_check"`

	// Status 状态（healthy, degraded, unhealthy）| Status (healthy, degraded, unhealthy)
	Status string `json:"status" msgpack:"status"`

	// GCCount GC执行次数 | GC execution count
	GCCount uint64 `json:"gc_count" msgpack:"gc_count"`
	// MemoryUsage 当前内存使用 | Current memory usage
	MemoryUsage uint64 `json:"memory_usage" msgpack:"memory_usage"`

	// ErrorRate 错误率 | Error rate
	ErrorRate float64 `json:"error_rate" msgpack:"error_rate"`

	// Uptime 运行时间 | Uptime duration
	Uptime time.Duration `json:"uptime" msgpack:"uptime"`
	// AvgDuration 平均执行时间 | Average execution duration
	AvgDuration time.Duration `json:"avg_duration" msgpack:"avg_duration"`
}

// Stats GC统计信息 | GC statistics
type Stats struct {
	// LastRunTime 最后执行时间 | Last run time
	LastRunTime time.Time `json:"last_run_time" msgpack:"last_run_time"`

	// LastRunDuration 最后执行耗时 | Last run duration
	LastRunDuration time.Duration `json:"last_run_duration" msgpack:"last_run_duration"`

	// AverageRunDuration 平均执行耗时 | Average run duration
	AverageRunDuration time.Duration `json:"average_run_duration" msgpack:"average_run_duration"`

	// TotalRuns 总执行次数 | Total run count
	TotalRuns uint64 `json:"total_runs" msgpack:"total_runs"`

	// SuccessfulRuns 成功执行次数 | Successful run count
	SuccessfulRuns uint64 `json:"successful_runs" msgpack:"successful_runs"`

	// FailedRuns 失败执行次数 | Failed run count
	FailedRuns uint64 `json:"failed_runs" msgpack:"failed_runs"`

	// MemoryBeforeGC GC前内存使用量（字节）| Memory usage before GC (bytes)
	MemoryBeforeGC uint64 `json:"memory_before_gc" msgpack:"memory_before_gc"`

	// MemoryAfterGC GC后内存使用量（字节）| Memory usage after GC (bytes)
	MemoryAfterGC uint64 `json:"memory_after_gc" msgpack:"memory_after_gc"`

	// MemoryFreed 释放的内存量（字节）| Memory freed (bytes)
	MemoryFreed uint64 `json:"memory_freed" msgpack:"memory_freed"`
}

// contextWrapper context包装器，用于池化 | Context wrapper for pooling
type contextWrapper struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Manager GC管理器 | GC manager
type Manager struct {
	ctxPool         sync.Pool
	lastHealthCheck time.Time
	strategy        Strategy
	ctx             context.Context
	cancel          context.CancelFunc
	config          *Config
	tuner           *DynamicTuner
	monitor         *PerformanceMonitor
	ticker          *time.Ticker
	stats           *Stats
	cleanupDone     chan struct{}
	done            chan struct{}
	healthStatus    HealthStatus
	mu              sync.RWMutex
	healthMu        sync.RWMutex
	tickerMu        sync.Mutex
	initialized     int32
	running         int32
}

// NewManager 获取全局单例GC管理器 | Get global singleton GC manager
func NewManager(config *Config) (*Manager, error) {
	// 使用读写锁保护单例创建过程
	singletonMu.Lock()
	defer singletonMu.Unlock()

	var initErr error

	managerOnce.Do(func() {
		logger.Info("Initializing global GC Manager singleton")

		if config == nil {
			initErr = errors.New("config cannot be nil")
			return
		}

		if err := config.Validate(); err != nil {
			initErr = fmt.Errorf("invalid config: %w", err)
			return
		}

		strategy, err := createStrategy(config.Strategy)
		if err != nil {
			initErr = fmt.Errorf("failed to create strategy: %w", err)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())

		globalGCManager = &Manager{
			config:      config,
			strategy:    strategy,
			stats:       new(Stats),
			ctx:         ctx,
			cancel:      cancel,
			done:        make(chan struct{}),
			cleanupDone: make(chan struct{}),
			initialized: 1,
			healthStatus: HealthStatus{
				Status:    "initializing",
				LastCheck: time.Now(),
				Details:   make(map[string]any),
			},
		}

		// 初始化性能监控器
		if config.EnableMonitoring {
			globalGCManager.monitor = NewPerformanceMonitor()
			// 设置配置到监控器，以便检查告警是否启用
			globalGCManager.monitor.SetConfig(config)
		}

		// 初始化动态调优器
		if config.EnableAutoTuning && globalGCManager.monitor != nil {
			globalGCManager.tuner = NewDynamicTuner(globalGCManager, globalGCManager.monitor)
		}

		// 初始化context池，使用配置化的超时时间
		timeout := config.ExecutionTimeout
		if timeout <= 0 {
			timeout = 60 * time.Second // 默认60秒
		}

		globalGCManager.ctxPool = sync.Pool{
			New: func() any {
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
				return &contextWrapper{ctx: ctx, cancel: cancel}
			},
		}

	})

	if initErr != nil {
		return nil, initErr
	}

	if globalGCManager == nil {
		return nil, errors.New("global GC Manager initialization failed")
	}

	return globalGCManager, nil
}

// GetGlobalManager 直接获取全局GC管理器实例（如果已初始化）| Get global GC manager instance directly (if initialized)
func GetGlobalManager() *Manager {
	return globalGCManager
}

// Start 启动GC管理器 | Start GC manager
func (m *Manager) Start() error {
	if m == nil {
		return errors.New("GC manager is nil")
	}

	// 检查是否已初始化
	if atomic.LoadInt32(&m.initialized) == 0 {
		return errors.New("GC manager not properly initialized")
	}

	if !m.config.Enabled {
		logger.Info("GC manager is disabled")
		return nil
	}

	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return ErrManagerAlreadyRunning
	}

	// 使用互斥锁保护ticker和done channel的操作
	m.tickerMu.Lock()
	defer m.tickerMu.Unlock()

	// 安全地停止旧的ticker
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.ticker = time.NewTicker(m.config.Interval)

	// 安全地重新创建done channel
	m.mu.Lock()
	select {
	case <-m.done:
		m.done = make(chan struct{})
	default:
		if m.done == nil {
			m.done = make(chan struct{})
		}
	}
	m.mu.Unlock()

	go m.run()

	// 启动自动调优
	if m.tuner != nil {
		go m.tuner.StartAutoTuning(m.ctx)
	}

	return nil
}

// Stop 停止GC管理器 | Stop GC manager
func (m *Manager) Stop() error {
	if m == nil {
		return errors.New("GC manager is nil")
	}

	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		return ErrManagerStopped
	}

	// 安全地取消context
	m.mu.RLock()
	cancel := m.cancel
	m.mu.RUnlock()

	if cancel != nil {
		cancel()
	}

	// 使用专用锁安全地停止ticker
	m.tickerMu.Lock()
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}
	m.tickerMu.Unlock()

	// 等待运行循环结束，使用更短的超时防止死锁
	m.mu.RLock()
	done := m.done
	cleanupDone := m.cleanupDone
	m.mu.RUnlock()

	if done != nil {
		select {
		case <-done:
			// 正常结束
		case <-time.After(500 * time.Millisecond):
			logger.Warn("GC manager stop timeout, forcing shutdown")
		}
	}

	// 清理context池中的资源
	if err := pool.Submit(func() {
		defer close(cleanupDone)

		// 清理context池中的所有context
		// 限制清理数量和时间，避免阻塞太久
		maxCleanup := 1000 // 最多清理1000个context
		cleaned := 0
		for cleaned < maxCleanup {
			if wrapper := m.ctxPool.Get(); wrapper != nil {
				if ctxWrapper, ok := wrapper.(*contextWrapper); ok && ctxWrapper.cancel != nil {
					ctxWrapper.cancel()
				}
				cleaned++
			} else {
				break
			}
		}
	}); err != nil {
		close(cleanupDone)
		logger.Warn("Submit context pool cleanup task failed", zap.Error(err))
	}

	// 等待清理完成，增加超时时间到5秒
	select {
	case <-cleanupDone:
		// 清理完成
	case <-time.After(5 * time.Second):
		logger.Warn("Context pool cleanup timeout, some contexts may not be cancelled")
	}

	logger.Warn("GC manager stopped")
	return nil
}

// ForceGC 手动触发GC | Manually trigger GC
func (m *Manager) ForceGC() error {
	if m == nil {
		return errors.New("GC manager is nil")
	}

	if atomic.LoadInt32(&m.initialized) == 0 {
		return errors.New("GC manager not properly initialized")
	}

	if atomic.LoadInt32(&m.running) == 0 {
		return ErrManagerStopped
	}

	return m.executeGC()
}

// GetStats 获取统计信息 | Get statistics
func (m *Manager) GetStats() Stats {
	if m == nil || m.stats == nil {
		return Stats{}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return *m.stats
}

// IsRunning 检查管理器是否正在运行 | Check if manager is running
func (m *Manager) IsRunning() bool {
	if m == nil {
		return false
	}
	return atomic.LoadInt32(&m.running) == 1
}

// run 主运行循环
func (m *Manager) run() {
	defer func() {
		// 恢复panic，防止goroutine崩溃影响整个程序
		if r := recover(); r != nil {
			logger.Error("GC manager run loop panic recovered", zap.Any("panic", r))
		}

		// 安全地关闭done channel
		m.mu.Lock()
		if m.done != nil {
			select {
			case <-m.done:
				// 已经关闭
			default:
				close(m.done)
			}
		}
		m.mu.Unlock()
	}()

	for {
		// 检查是否仍在运行状态
		if atomic.LoadInt32(&m.running) == 0 {
			return
		}

		// 安全地获取context和ticker
		m.mu.RLock()
		ctx := m.ctx
		m.mu.RUnlock()

		m.tickerMu.Lock()
		ticker := m.ticker
		m.tickerMu.Unlock()

		if ctx == nil || ticker == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 再次检查是否仍在运行状态
			if atomic.LoadInt32(&m.running) == 0 {
				return
			}

			// 使用协程池异步执行GC，避免阻塞主循环
			err := pool.Submit(func() {
				if err := m.executeGC(); err != nil {
					logger.Error("GC execution failed", zap.Error(err))
				}
			})
			if err != nil {
				// 降级处理：同步执行
				if err := m.executeGC(); err != nil {
					logger.Error("GC execution failed", zap.Error(err))
				}
			}
		}
	}
}

// executeGC 执行GC操作
func (m *Manager) executeGC() error {
	// 防御性检查
	if m == nil {
		return errors.New("GC manager is nil")
	}

	// 使用读锁保护对共享资源的访问
	m.mu.RLock()
	strategy := m.strategy
	config := m.config
	m.mu.RUnlock()

	if strategy == nil || config == nil {
		return errors.New("GC manager components not properly initialized")
	}

	start := time.Now()

	// 记录GC前的内存状态
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// 使用retry机制执行GC，增加超时控制
	backoff := retry.WithMaxRetries(uint64(config.MaxRetries),
		retry.MustNewConstant(config.RetryInterval))

	// 从池中获取context，提高性能
	ctxWrapper := m.ctxPool.Get().(*contextWrapper)
	defer func() {
		if ctxWrapper.cancel != nil {
			ctxWrapper.cancel() // 取消context
		}
		// 重新创建context放回池中，使用配置的超时时间
		timeout := config.ExecutionTimeout
		if timeout <= 0 {
			timeout = 60 * time.Second // 默认60秒
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		ctxWrapper.ctx = ctx
		ctxWrapper.cancel = cancel
		m.ctxPool.Put(ctxWrapper)
	}()

	gcCtx := ctxWrapper.ctx

	err := retry.Do(gcCtx, backoff, func(ctx context.Context) error {
		// 检查是否仍在运行状态
		if atomic.LoadInt32(&m.running) == 0 {
			return errors.New("GC manager stopped during execution")
		}

		return retry.RetryableError(strategy.Execute(ctx))
	})

	// 记录GC后的内存状态
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	duration := time.Since(start)

	// 更新统计信息
	m.updateStats(err == nil, duration, memBefore.Alloc, memAfter.Alloc)

	// 记录性能监控数据
	if m.monitor != nil {
		m.monitor.RecordGCExecution(duration, memBefore.Alloc, memAfter.Alloc)
	}

	if err != nil {
		// 区分超时错误和其他错误
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("GC execution timeout - consider increasing execution_timeout",
				zap.Error(err),
				zap.Duration("duration", duration),
				zap.Duration("timeout", m.config.ExecutionTimeout),
				zap.String("strategy", m.strategy.Name()),
				zap.String("suggestion", "increase execution_timeout in config or switch to gentle strategy"))
		} else {
			logger.Error("GC execution failed after retries",
				zap.Error(err),
				zap.Duration("duration", duration))
		}
		return err
	}

	// 只在统计启用且执行时间较长时记录详细日志
	if m.config.IsStatsEnabled() && duration > 500*time.Millisecond {
		var memoryFreed uint64
		if memBefore.Alloc > memAfter.Alloc {
			memoryFreed = memBefore.Alloc - memAfter.Alloc
		} else {
			memoryFreed = 0
		}

		logger.Info("GC executed successfully",
			zap.String("strategy", m.strategy.Name()),
			zap.Duration("duration", duration),
			zap.Uint64("memory_before", memBefore.Alloc),
			zap.Uint64("memory_after", memAfter.Alloc),
			zap.Uint64("memory_freed", memoryFreed))
	}

	return nil
}

// updateStats 更新统计信息
func (m *Manager) updateStats(success bool, duration time.Duration, memBefore, memAfter uint64) {
	if m == nil || m.stats == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalRuns++
	m.stats.LastRunTime = time.Now()
	m.stats.LastRunDuration = duration
	m.stats.MemoryBeforeGC = memBefore
	m.stats.MemoryAfterGC = memAfter

	if memBefore > memAfter {
		m.stats.MemoryFreed = memBefore - memAfter
	} else {
		m.stats.MemoryFreed = 0
	}

	if success {
		m.stats.SuccessfulRuns++
	} else {
		m.stats.FailedRuns++
	}

	// 计算平均执行时间，防止溢出
	if m.stats.TotalRuns > 0 {
		// 使用更安全的计算方式
		prevAvg := m.stats.AverageRunDuration.Nanoseconds()
		newAvg := (prevAvg*int64(m.stats.TotalRuns-1) + duration.Nanoseconds()) / int64(m.stats.TotalRuns)
		m.stats.AverageRunDuration = time.Duration(newAvg)
	}
}

// createStrategy 创建GC策略
func createStrategy(strategyType StrategyType) (Strategy, error) {
	switch strategyType {
	case StrategyStandard:
		return new(StandardStrategy), nil
	case StrategyAggressive:
		return new(AggressiveStrategy), nil
	case StrategyGentle:
		// Go 1.26+ new(expr) 语法 | Go 1.26+ new(expr) syntax
		return &GentleStrategy{threshold: 50 * 1024 * 1024}, nil // 50MB默认阈值
	case StrategyAdaptive:
		return new(AdaptiveStrategy), nil
	case StrategyPressureAware:
		return NewPressureAwareStrategy(), nil
	case StrategyScheduled:
		return NewScheduledStrategy(), nil
	default:
		return nil, ErrInvalidStrategy
	}
}

// createStrategyWithConfig 根据配置创建GC策略
func createStrategyWithConfig(strategyType StrategyType, config *Config) (Strategy, error) {
	switch strategyType {
	case StrategyStandard:
		return new(StandardStrategy), nil
	case StrategyAggressive:
		return new(AggressiveStrategy), nil
	case StrategyGentle:
		// 使用配置中的内存阈值
		threshold := config.MemoryThreshold
		if threshold == 0 {
			threshold = 50 * 1024 * 1024 // 默认50MB
		}
		return &GentleStrategy{threshold: threshold}, nil
	case StrategyAdaptive:
		return new(AdaptiveStrategy), nil
	case StrategyPressureAware:
		return NewPressureAwareStrategy(), nil
	case StrategyScheduled:
		return NewScheduledStrategy(), nil
	default:
		return nil, ErrInvalidStrategy
	}
}

// StandardStrategy 标准GC策略 | Standard GC strategy
type StandardStrategy struct{}

// Execute 执行标准GC | Execute standard GC
func (s *StandardStrategy) Execute(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		runtime.GC()
		return nil
	}
}

// Name 返回策略名称 | Return strategy name
func (s *StandardStrategy) Name() string {
	return "standard"
}

// AggressiveStrategy 激进GC策略 | Aggressive GC strategy
type AggressiveStrategy struct{}

// Execute 执行激进GC | Execute aggressive GC
func (s *AggressiveStrategy) Execute(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 执行标准GC
		runtime.GC()

		// 强制释放操作系统内存
		debug.FreeOSMemory()

		return nil
	}
}

// Name 返回策略名称 | Return strategy name
func (s *AggressiveStrategy) Name() string {
	return "aggressive"
}

// GentleStrategy 温和GC策略 | Gentle GC strategy
type GentleStrategy struct {
	threshold uint64 // 可配置的内存阈值
}

// Execute 执行温和GC | Execute gentle GC
func (s *GentleStrategy) Execute(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 检查内存使用情况，只在必要时执行GC
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// 使用可配置的阈值，如果未设置则使用默认值
		threshold := s.threshold
		if threshold == 0 {
			threshold = 50 * 1024 * 1024 // 50MB默认阈值
		}

		// 如果堆内存使用超过阈值才执行GC
		if m.HeapAlloc > threshold {
			runtime.GC()
		}

		return nil
	}
}

// Name 返回策略名称 | Return strategy name
func (s *GentleStrategy) Name() string {
	return "gentle"
}

// AdaptiveStrategy 自适应GC策略 | Adaptive GC strategy
type AdaptiveStrategy struct {
	lastGCTime   time.Time
	lastHeapSize uint64
	gcFrequency  time.Duration
	mu           sync.RWMutex
}

// Execute 执行自适应GC | Execute adaptive GC
func (s *AdaptiveStrategy) Execute(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		s.mu.Lock()
		defer s.mu.Unlock()

		now := time.Now()

		// 自适应逻辑：根据内存增长速度调整GC策略
		if !s.lastGCTime.IsZero() {
			timeSinceLastGC := now.Sub(s.lastGCTime)

			// 安全计算内存增长，避免溢出
			var memoryGrowth uint64
			if m.HeapAlloc > s.lastHeapSize {
				memoryGrowth = m.HeapAlloc - s.lastHeapSize
			} else {
				memoryGrowth = 0 // 内存减少或相等
			}

			// 如果内存增长快，增加GC频率
			if memoryGrowth > 10*1024*1024 && timeSinceLastGC < 30*time.Second {
				runtime.GC()
				debug.FreeOSMemory() // 激进清理
			} else if memoryGrowth > 5*1024*1024 {
				runtime.GC()
			}
			// 如果内存增长慢，可以跳过这次GC
		} else {
			// 首次执行
			runtime.GC()
		}

		s.lastGCTime = now
		s.lastHeapSize = m.HeapAlloc

		return nil
	}
}

// Name 返回策略名称 | Return strategy name
func (s *AdaptiveStrategy) Name() string {
	return "adaptive"
}

// Service GC服务，用于集成到主应用中 | GC service for integration into main application
type Service struct {
	manager *Manager
}

// NewService 获取全局单例GC服务 | Get global singleton GC service
func NewService(config *Config) (*Service, error) {
	var initErr error

	serviceOnce.Do(func() {
		logger.Info("Initializing global GC Service singleton")

		manager, err := NewManager(config)
		if err != nil {
			initErr = err
			return
		}

		globalGCService = &Service{
			manager: manager,
		}

		logger.Info("Global GC Service initialized successfully")
	})

	if initErr != nil {
		return nil, initErr
	}

	if globalGCService == nil {
		return nil, errors.New("global GC Service initialization failed")
	}

	return globalGCService, nil
}

// GetGlobalService 直接获取全局GC服务实例（如果已初始化）| Get global GC service instance directly (if initialized)
func GetGlobalService() *Service {
	singletonMu.RLock()
	defer singletonMu.RUnlock()
	return globalGCService
}

// resetSingletonForTesting 重置单例实例（仅用于测试）
func resetSingletonForTesting() {
	singletonMu.Lock()
	defer singletonMu.Unlock()

	// 停止现有的管理器
	if globalGCManager != nil && globalGCManager.IsRunning() {
		_ = globalGCManager.Stop()
	}

	// 重置全局变量
	globalGCManager = nil
	globalGCService = nil
	managerOnce = sync.Once{}
	serviceOnce = sync.Once{}

	// 重置监控器和调优器
	resetMonitorForTesting()
	resetTunerForTesting()
	resetStrategiesForTesting()
}

// Start 启动GC服务 | Start GC service
func (s *Service) Start() error {
	if s == nil || s.manager == nil {
		return errors.New("GC service not properly initialized")
	}
	return s.manager.Start()
}

// Stop 停止GC服务 | Stop GC service
func (s *Service) Stop() error {
	if s == nil || s.manager == nil {
		return errors.New("GC service not properly initialized")
	}
	return s.manager.Stop()
}

// ForceGC 手动触发GC | Manually trigger GC
func (s *Service) ForceGC() error {
	if s == nil || s.manager == nil {
		return errors.New("GC service not properly initialized")
	}
	return s.manager.ForceGC()
}

// GetStats 获取统计信息 | Get statistics
func (s *Service) GetStats() Stats {
	if s == nil || s.manager == nil {
		return Stats{}
	}
	return s.manager.GetStats()
}

// IsRunning 检查服务是否正在运行 | Check if service is running
func (s *Service) IsRunning() bool {
	if s == nil || s.manager == nil {
		return false
	}
	return s.manager.IsRunning()
}

// GetMonitor 获取性能监控器 | Get performance monitor
func (s *Service) GetMonitor() *PerformanceMonitor {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.monitor
}

// GetTuner 获取动态调优器 | Get dynamic tuner
func (s *Service) GetTuner() *DynamicTuner {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.tuner
}

// GetHealthStatus 获取健康状态 | Get health status
func (s *Service) GetHealthStatus() HealthStatus {
	if s == nil || s.manager == nil {
		return HealthStatus{
			Status:    "unhealthy",
			LastCheck: time.Now(),
			Details:   map[string]any{"error": "service or manager is nil"},
		}
	}
	return s.manager.GetHealthStatus()
}

// ReloadConfig 热重载配置 | Hot reload configuration
func (s *Service) ReloadConfig(newConfig *Config) error {
	if s == nil || s.manager == nil {
		return errors.New("service or manager is nil")
	}
	return s.manager.ReloadConfig(newConfig)
}

// GetConfig 获取当前配置 | Get current configuration
func (s *Service) GetConfig() *Config {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.GetConfig()
}

// EnableGracefulDegradation 启用优雅降级 | Enable graceful degradation
func (s *Service) EnableGracefulDegradation() {
	if s == nil || s.manager == nil {
		return
	}
	s.manager.EnableGracefulDegradation()
}

// DisableGracefulDegradation 禁用优雅降级 | Disable graceful degradation
func (s *Service) DisableGracefulDegradation(originalConfig *Config) error {
	if s == nil || s.manager == nil {
		return errors.New("service or manager is nil")
	}
	return s.manager.DisableGracefulDegradation(originalConfig)
}

// NewServiceFromAppConfig 从应用配置创建GC服务 | Create GC service from application config
func NewServiceFromAppConfig(appGC any) (*Service, error) {
	// 类型断言，支持不同的配置类型
	var config *Config

	switch gc := appGC.(type) {
	case *Config:
		config = gc
	case map[string]any:
		// 从map创建配置
		config = &Config{
			Enabled:          getBoolFromMap(gc, "enabled", true),
			Interval:         getDurationFromMap(gc, "interval", 5*time.Minute),
			MemoryThreshold:  getUint64FromMap(gc, "memory_threshold", 100*1024*1024),
			Strategy:         StrategyType(getStringFromMap(gc, "strategy", "standard")),
			ForceGC:          getBoolFromMap(gc, "force_gc", false),
			FreeOSMemory:     getBoolFromMap(gc, "free_os_memory", false),
			EnableStats:      getBoolFromMap(gc, "enable_stats", true),
			EnableMonitoring: getBoolFromMap(gc, "enable_monitoring", true),
			EnableAutoTuning: getBoolFromMap(gc, "enable_auto_tuning", false),
			EnableAlerts:     getBoolFromMap(gc, "enable_alerts", true),
			MaxRetries:       getIntFromMap(gc, "max_retries", 2),
			RetryInterval:    getDurationFromMap(gc, "retry_interval", 10*time.Second),
			ExecutionTimeout: getDurationFromMap(gc, "execution_timeout", 60*time.Second),
		}
	default:
		// 使用默认配置
		config = DefaultConfig()
	}

	return NewService(config)
}

// 辅助函数：从map中获取bool值
func getBoolFromMap(m map[string]any, key string, defaultValue bool) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// 辅助函数：从map中获取string值
func getStringFromMap(m map[string]any, key string, defaultValue string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultValue
}

// 辅助函数：从map中获取int值
func getIntFromMap(m map[string]any, key string, defaultValue int) int {
	if val, ok := m[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
		if f, ok := val.(float64); ok {
			return int(f)
		}
	}
	return defaultValue
}

// 辅助函数：从map中获取uint64值
func getUint64FromMap(m map[string]any, key string, defaultValue uint64) uint64 {
	if val, ok := m[key]; ok {
		if u, ok := val.(uint64); ok {
			return u
		}
		if f, ok := val.(float64); ok {
			return uint64(f)
		}
		if i, ok := val.(int); ok {
			return uint64(i)
		}
	}
	return defaultValue
}

// 辅助函数：从map中获取duration值
func getDurationFromMap(m map[string]any, key string, defaultValue time.Duration) time.Duration {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			if d, err := time.ParseDuration(s); err == nil {
				return d
			}
		}
		if i, ok := val.(int64); ok {
			return time.Duration(i)
		}
		if f, ok := val.(float64); ok {
			return time.Duration(f)
		}
	}
	return defaultValue
}

// AppGCConfig 从应用配置转换为GC配置 | Application GC config interface for converting to GC config
type AppGCConfig interface {
	IsEnabled() bool
	GetInterval() time.Duration
	GetMemoryThreshold() uint64
	GetStrategy() string
	ShouldForceGC() bool
	ShouldFreeOSMemory() bool
	IsStatsEnabled() bool
	IsMonitoringEnabled() bool
	IsAutoTuningEnabled() bool
	IsAlertsEnabled() bool
	GetMaxRetries() int
	GetRetryInterval() time.Duration
	GetExecutionTimeout() time.Duration
	GetPressureThresholds() any // 使用any以支持不同的阈值类型 | Use any to support different threshold types
}

// NewServiceFromAppGCConfig 从应用GC配置创建服务 | Create service from application GC config
func NewServiceFromAppGCConfig(appGC AppGCConfig) (*Service, error) {
	if appGC == nil {
		// 使用默认配置
		return NewService(DefaultConfig())
	}

	config := &Config{
		Enabled:          appGC.IsEnabled(),
		Interval:         appGC.GetInterval(),
		MemoryThreshold:  appGC.GetMemoryThreshold(),
		Strategy:         StrategyType(appGC.GetStrategy()),
		ForceGC:          appGC.ShouldForceGC(),
		FreeOSMemory:     appGC.ShouldFreeOSMemory(),
		EnableStats:      appGC.IsStatsEnabled(),
		EnableMonitoring: appGC.IsMonitoringEnabled(),
		EnableAutoTuning: appGC.IsAutoTuningEnabled(),
		EnableAlerts:     appGC.IsAlertsEnabled(),
		MaxRetries:       appGC.GetMaxRetries(),
		RetryInterval:    appGC.GetRetryInterval(),
		ExecutionTimeout: appGC.GetExecutionTimeout(),
	}

	// 如果ExecutionTimeout为0，使用默认值
	if config.ExecutionTimeout <= 0 {
		config.ExecutionTimeout = 60 * time.Second
	}

	// 处理压力阈值配置
	if thresholds := appGC.GetPressureThresholds(); thresholds != nil {
		config.PressureThresholds = convertPressureThresholds(thresholds)
	}

	return NewService(config)
}

// convertPressureThresholds 转换压力阈值配置
func convertPressureThresholds(thresholds any) *PressureThresholds {
	switch t := thresholds.(type) {
	case *PressureThresholds:
		return t
	case map[string]any:
		return &PressureThresholds{
			Low:      getUint64FromMap(t, "low", 50*1024*1024),
			Medium:   getUint64FromMap(t, "medium", 100*1024*1024),
			High:     getUint64FromMap(t, "high", 200*1024*1024),
			Critical: getUint64FromMap(t, "critical", 500*1024*1024),
		}
	default:
		// 尝试通过反射处理其他类型（如config包的PressureThresholds）
		if hasFields(thresholds, "Low", "Medium", "High", "Critical") {
			return &PressureThresholds{
				Low:      getFieldUint64(thresholds, "Low", 50*1024*1024),
				Medium:   getFieldUint64(thresholds, "Medium", 100*1024*1024),
				High:     getFieldUint64(thresholds, "High", 200*1024*1024),
				Critical: getFieldUint64(thresholds, "Critical", 500*1024*1024),
			}
		}

		// 返回默认阈值
		return &PressureThresholds{
			Low:      50 * 1024 * 1024,
			Medium:   100 * 1024 * 1024,
			High:     200 * 1024 * 1024,
			Critical: 500 * 1024 * 1024,
		}
	}
}

// hasFields 检查对象是否有指定的字段
func hasFields(obj any, fieldNames ...string) bool {
	if obj == nil {
		return false
	}

	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// 只处理结构体类型，其他类型（如map）直接返回false
	if v.Kind() != reflect.Struct {
		return false
	}

	for _, fieldName := range fieldNames {
		if !v.FieldByName(fieldName).IsValid() {
			return false
		}
	}

	return true
}

// getFieldUint64 通过反射获取字段的uint64值
func getFieldUint64(obj any, fieldName string, defaultValue uint64) uint64 {
	if obj == nil {
		return defaultValue
	}

	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// 只处理结构体类型，其他类型（如map）直接返回默认值
	if v.Kind() != reflect.Struct {
		return defaultValue
	}

	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return defaultValue
	}

	switch field.Kind() {
	case reflect.Uint64:
		return field.Uint()
	case reflect.Uint, reflect.Uint32:
		return uint64(field.Uint())
	case reflect.Int, reflect.Int32, reflect.Int64:
		val := field.Int()
		if val >= 0 {
			return uint64(val)
		}
		return defaultValue
	default:
		// 不支持的类型，返回默认值
		return defaultValue
	}

}

// GetHealthStatus 获取健康状态 | Get health status
func (m *Manager) GetHealthStatus() HealthStatus {
	if m == nil {
		return HealthStatus{
			Status:    "unhealthy",
			LastCheck: time.Now(),
			Details:   map[string]any{"error": "manager is nil"},
		}
	}

	m.healthMu.RLock()
	defer m.healthMu.RUnlock()

	// 如果健康检查过期，执行新的检查
	if time.Since(m.lastHealthCheck) > time.Minute {
		m.healthMu.RUnlock()
		m.performHealthCheck()
		m.healthMu.RLock()
	}

	return m.healthStatus
}

// performHealthCheck 执行健康检查
func (m *Manager) performHealthCheck() {
	m.healthMu.Lock()
	defer m.healthMu.Unlock()

	now := time.Now()
	m.lastHealthCheck = now

	// 安全读取统计信息
	m.mu.RLock()
	var uptime time.Duration
	var errorRate float64
	var avgDuration time.Duration
	var totalRuns uint64

	if m.stats != nil {
		if !m.stats.LastRunTime.IsZero() {
			uptime = now.Sub(m.stats.LastRunTime)
		}
		if m.stats.TotalRuns > 0 {
			errorRate = float64(m.stats.FailedRuns) / float64(m.stats.TotalRuns)
		}
		avgDuration = m.stats.AverageRunDuration
		totalRuns = m.stats.TotalRuns
	}
	m.mu.RUnlock()

	// 获取当前内存使用
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 确定健康状态
	status := "healthy"
	details := make(map[string]any)

	// 保留现有的降级状态
	if m.healthStatus.Status == "degraded" {
		if degradationMode, exists := m.healthStatus.Details["degradation_mode"]; exists {
			status = "degraded"
			details["degradation_mode"] = degradationMode
		}
	}

	if errorRate > 0.1 { // 错误率超过10%
		status = "degraded"
		details["high_error_rate"] = errorRate
	}

	if avgDuration > 10*time.Second { // 平均执行时间超过10秒
		status = "degraded"
		details["slow_execution"] = avgDuration
	}

	if !m.IsRunning() {
		status = "unhealthy"
		details["not_running"] = true
	}

	// 更新健康状态
	m.healthStatus = HealthStatus{
		Status:      status,
		LastCheck:   now,
		Uptime:      uptime,
		GCCount:     totalRuns,
		ErrorRate:   errorRate,
		AvgDuration: avgDuration,
		MemoryUsage: memStats.HeapAlloc,
		Details:     details,
	}
}

// ReloadConfig 热重载配置 | Hot reload configuration
func (m *Manager) ReloadConfig(newConfig *Config) error {
	if m == nil {
		return errors.New("manager is nil")
	}

	if newConfig == nil {
		return errors.New("new config is nil")
	}

	// 验证新配置
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 保存旧配置用于回滚
	oldConfig := m.config

	// 创建新策略
	newStrategy, err := createStrategyWithConfig(newConfig.Strategy, newConfig)
	if err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// 更新配置和策略
	m.config = newConfig
	m.strategy = newStrategy

	// 更新监控器配置
	if m.monitor != nil {
		m.monitor.SetConfig(newConfig)
	}

	// 如果间隔时间改变，需要重新创建ticker
	if oldConfig.Interval != newConfig.Interval && m.IsRunning() {
		m.tickerMu.Lock()
		if m.ticker != nil {
			m.ticker.Stop()
			m.ticker = time.NewTicker(newConfig.Interval)
		}
		m.tickerMu.Unlock()
	}

	logger.Info("GC configuration reloaded successfully",
		zap.String("old_strategy", string(oldConfig.Strategy)),
		zap.String("new_strategy", string(newConfig.Strategy)),
		zap.Duration("old_interval", oldConfig.Interval),
		zap.Duration("new_interval", newConfig.Interval))

	return nil
}

// GetConfig 获取当前配置（只读副本）| Get current configuration (read-only copy)
func (m *Manager) GetConfig() *Config {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil
	}

	// 返回配置的副本，避免外部修改
	configCopy := *m.config
	return &configCopy
}

// EnableGracefulDegradation 启用优雅降级模式 | Enable graceful degradation mode
func (m *Manager) EnableGracefulDegradation() {
	if m == nil {
		return
	}

	m.healthMu.Lock()
	defer m.healthMu.Unlock()

	// 切换到温和策略
	gentleStrategy := &GentleStrategy{threshold: 100 * 1024 * 1024} // 100MB阈值

	m.mu.Lock()
	m.strategy = gentleStrategy
	// 增加GC间隔，减少频率
	if m.config.Interval < 10*time.Minute {
		m.config.Interval = 10 * time.Minute

		// 更新ticker
		if m.IsRunning() {
			m.tickerMu.Lock()
			if m.ticker != nil {
				m.ticker.Stop()
				m.ticker = time.NewTicker(m.config.Interval)
			}
			m.tickerMu.Unlock()
		}
	}
	m.mu.Unlock()

	// 更新健康状态
	m.healthStatus.Status = "degraded"
	m.healthStatus.Details["degradation_mode"] = "enabled"

	logger.Warn("GC manager entered graceful degradation mode",
		zap.String("strategy", "gentle"),
		zap.Duration("interval", m.config.Interval))
}

// DisableGracefulDegradation 禁用优雅降级模式 | Disable graceful degradation mode
func (m *Manager) DisableGracefulDegradation(originalConfig *Config) error {
	if m == nil {
		return errors.New("manager is nil")
	}

	if originalConfig == nil {
		return errors.New("original config is nil")
	}

	// 恢复原始配置
	return m.ReloadConfig(originalConfig)
}
