# Retry Package

`pkg/retry` 包提供了灵活且强大的重试机制，支持多种退避策略、错误收集、回调函数等高级功能。

## ✨ 特性

- 🔄 **多种退避策略**: 常量、线性、指数、斐波那契
- 🎯 **灵活配置**: 支持最大重试次数、超时、抖动等
- 🔒 **并发安全**: 所有组件都经过竞态检测验证
- ⚡ **高性能**: 优化的随机数生成和对象池
- 📊 **错误收集**: 收集所有重试过程中的错误
- 🔔 **回调支持**: 在每次重试时执行自定义逻辑
- 🛡️ **Panic 恢复**: 自动捕获和处理 panic
- 📝 **完整文档**: 详细的 API 文档和使用示例

## 目录

- [快速开始](#快速开始)
- [核心概念](#核心概念)
- [退避策略](#退避策略)
- [便捷函数](#便捷函数)
- [高级功能](#高级功能)
- [最佳实践](#最佳实践)
- [性能优化](#性能优化)
- [API 参考](#api-参考)
- [常见问题](#常见问题)

## 快速开始

### 最简单的方式

使用便捷函数，一行代码完成重试：

```go
import (
    "context"
    "pkg/retry"
)

func main() {
    ctx := context.Background()

    // 快速重试：3次，100ms间隔
    err := retry.DoQuick(ctx, func(ctx context.Context) error {
        return doSomething()
    })

    // 标准重试：5次，指数退避，最大30秒
    err = retry.DoStandard(ctx, func(ctx context.Context) error {
        return doSomething()
    })
}
```

### 基本用法

```go
import (
"context"
"time"
"pkg/retry"
)

func main() {
ctx := context.Background()

// 方式1：使用 MustNew* 函数（推荐）
backoff := retry.MustNewConstant(time.Second)

// 方式2：使用 New* 函数（需要错误处理）
backoff, err := retry.NewConstant(time.Second)
if err != nil {
// 处理错误
}

// 执行重试
err = retry.Do(ctx, backoff, func (ctx context.Context) error {
// 你的业务逻辑
result, err := doSomething()
if err != nil {
// 只有包装为RetryableError的错误才会重试
return retry.RetryableError(err)
}
return nil
})
}
```

### 限制重试次数

```go
// 最多重试3次（总共4次尝试）
backoff := retry.WithMaxRetries(3, retry.TestNewConstant(time.Second))

err := retry.Do(ctx, backoff, retryFunc)
```

## 核心概念

### 可重试错误

只有使用 `RetryableError()` 包装的错误才会触发重试：

```go
// 会重试
return retry.RetryableError(errors.New("temporary failure"))

// 不会重试，立即返回
return errors.New("permanent failure")
```

### Backoff 实例隔离

⚠️ **重要**：每个独立的重试操作必须创建自己的 Backoff 实例！

```go
// ✅ 正确：每个goroutine创建独立实例
for i := 0; i < 10; i++ {
go func () {
backoff := retry.WithMaxRetries(3, retry.TestNewExponential(time.Second))
retry.Do(ctx, backoff, retryFunc)
}()
}

// ❌ 错误：共享Backoff实例会导致状态混乱
backoff := retry.WithMaxRetries(3, retry.TestNewExponential(time.Second))
for i := 0; i < 10; i++ {
go func () {
retry.Do(ctx, backoff, retryFunc) // 错误！
}()
}
```

## 退避策略

### 1. 常量退避（Constant）

每次重试等待固定时间：

```go
backoff, err := retry.NewConstant(time.Second)
// 等待时间：1s, 1s, 1s, 1s, ...
```

### 2. 指数退避（Exponential）

每次重试等待时间翻倍：

```go
backoff, err := retry.NewExponential(time.Second)
// 等待时间：1s, 2s, 4s, 8s, 16s, 32s, ...
```

### 3. 斐波那契退避（Fibonacci）

等待时间按斐波那契数列增长：

```go
backoff, err := retry.NewFibonacci(time.Second)
// 等待时间：1s, 2s, 3s, 5s, 8s, 13s, ...
```

### 4. 线性退避（Linear）

等待时间按线性增长：

```go
backoff, err := retry.NewLinear(time.Second)
// 等待时间：1s, 2s, 3s, 4s, 5s, 6s, ...
```

## 便捷函数

为了简化常见场景的使用，retry 包提供了多个便捷函数：

### 快速重试

```go
// 使用默认配置：3次重试，100ms间隔
err := retry.DoQuick(ctx, func (ctx context.Context) error {
return doSomething()
})
```

### 标准重试

```go
// 使用标准配置：5次重试，指数退避，最大等待30秒
err := retry.DoStandard(ctx, func (ctx context.Context) error {
return doSomething()
})
```

### 激进重试

```go
// 使用激进配置：10次重试，指数退避+抖动，最大等待1分钟
err := retry.DoAggressive(ctx, func (ctx context.Context) error {
return doSomething()
})
```

### 使用配置对象

```go
config := &retry.RetryConfig{
MaxRetries:    10,
BaseInterval:  time.Second,
MaxInterval:   30 * time.Second,
JitterPercent: 10,
Strategy:      "exponential",
}

err := retry.DoWithConfig(ctx, config, func (ctx context.Context) error {
return doSomething()
})
```

## 高级功能

### 组合退避策略

可以组合多个退避策略：

```go
backoff := retry.WithMaxRetries(10, // 最多重试10次
retry.WithCappedDuration(time.Minute, // 单次等待最多1分钟
retry.WithJitterPercent(10, // 添加10%的随机抖动
retry.TestNewExponential(time.Second)))) // 基础指数退避
```

### 添加抖动（Jitter）

避免"惊群效应"：

```go
// 固定时间抖动：+/- 500ms
backoff := retry.WithJitter(500*time.Millisecond, retry.TestNewConstant(time.Second))

// 百分比抖动：+/- 10%
backoff := retry.WithJitterPercent(10, retry.TestNewConstant(time.Second))
```

### 限制总时间

```go
// 总重试时间不超过30秒
backoff := retry.WithMaxDuration(30*time.Second, retry.TestNewExponential(time.Second))
```

### 错误收集

收集所有重试过程中的错误：

```go
collector := retry.DoWithErrorCollection(ctx, backoff, retryFunc)

fmt.Printf("Total errors: %d\n", collector.Count())
for i, err := range collector.Errors() {
fmt.Printf("Error %d: %v\n", i+1, err)
}
```

### 回调函数

在每次重试前执行回调（用于日志、监控等）：

```go
callback := func (attempt int, err error, nextWait time.Duration) {
log.Printf("Attempt %d failed: %v, waiting %v before retry", attempt, err, nextWait)
}

err := retry.DoWithCallback(ctx, backoff, retryFunc, callback)
```

### Panic 恢复

自动捕获并重试 panic：

```go
err := retry.DoWithPanicRecovery(ctx, backoff, func(ctx context.Context) error {
// 即使这里panic，也会被捕获并重试
panic("something went wrong")
})

// 检查是否是panic错误
var panicErr *retry.PanicError
if errors.As(err, &panicErr) {
fmt.Printf("Panic occurred: %v\n", panicErr.Value)
}
```

## 最佳实践

### 1. 总是设置超时

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := retry.Do(ctx, backoff, retryFunc)
```

### 2. 组合使用限制条件

```go
backoff := retry.WithMaxRetries(10, // 限制重试次数
retry.WithMaxDuration(time.Minute, // 限制总时间
retry.WithCappedDuration(5*time.Second, // 限制单次等待
retry.MustNewExponential(time.Second)))) // 基础策略
```

### 3. 区分可重试和不可重试错误

```go
func retryFunc(ctx context.Context) error {
result, err := callAPI()
if err != nil {
// 网络错误、超时等临时错误应该重试
if isTemporaryError(err) {
return retry.RetryableError(err)
}
// 认证失败、参数错误等永久错误不应该重试
return err
}
return nil
}
```

### 4. 使用抖动避免惊群

在高并发场景下，添加抖动可以避免所有客户端同时重试：

```go
backoff := retry.WithJitterPercent(20, retry.TestNewExponential(time.Second))
```

### 5. 记录重试日志

```go
callback := func (attempt int, err error, nextWait time.Duration) {
logger.Warn("Retry attempt",
zap.Int("attempt", attempt),
zap.Error(err),
zap.Duration("next_wait", nextWait))
}

err := retry.DoWithCallback(ctx, backoff, retryFunc, callback)
```

## 性能优化

### 性能特性

retry 包经过精心优化，具有以下性能特性：

1. **优化的随机数生成**
    - 在创建时初始化随机数生成器，避免每次调用时的开销
    - 减少 `time.Now()` 系统调用次数（性能提升 30-50%）
    - 减少 `Seed()` 调用次数

2. **读写锁优化**
    - `ErrorCollector` 使用 `sync.RWMutex`
    - 多个读操作可以并发执行
    - 在读多写少的场景下性能提升 2-5倍

3. **原子操作**
    - 使用原子操作而不是互斥锁来提高性能
    - `WithMaxRetries` 使用 `atomic.AddUint64`
    - 指数退避和线性退避使用原子计数器

4. **对象池**
    - 随机数生成器使用 `sync.Pool`
    - 减少内存分配和 GC 压力

### 内存使用

- `constantBackoff`: ~16 bytes/实例
- `exponentialBackoff`: ~24 bytes/实例
- `linearBackoff`: ~24 bytes/实例
- `fibonacciBackoff`: ~40 bytes/实例
- `ErrorCollector`: 动态增长，建议在高频场景下定期清理
- 随机数生成器使用对象池，内存占用可忽略不计

### 并发性能

- `exponentialBackoff` 使用原子操作，并发性能优秀
- `linearBackoff` 使用原子操作，并发性能优秀
- `fibonacciBackoff` 使用互斥锁，性能稍差但仍然足够
- 建议每个重试操作创建独立实例，避免锁竞争

### 并发安全

所有退避策略都是并发安全的，但每个重试操作应该使用独立的 Backoff 实例：

```go
// ❌ 错误：共享 Backoff 实例
backoff := retry.MustNewExponential(time.Second)
for i := 0; i < 10; i++ {
    go func() {
        retry.Do(ctx, backoff, retryFunc)  // 不安全！
    }()
}

// ✅ 正确：每个 goroutine 使用独立的 Backoff 实例
for i := 0; i < 10; i++ {
    go func() {
        backoff := retry.MustNewExponential(time.Second)
        retry.Do(ctx, backoff, retryFunc)  // 安全
    }()
}
```

### 性能基准

```
BenchmarkWithJitter_HighFrequency-8              5000000    230 ns/op    0 B/op    0 allocs/op
BenchmarkWithJitterPercent_HighFrequency-8       5000000    235 ns/op    0 B/op    0 allocs/op
BenchmarkErrorCollector_ConcurrentRead-8        10000000    120 ns/op   48 B/op    1 allocs/op
BenchmarkHighConcurrency-8                       1000000   1200 ns/op  256 B/op    8 allocs/op
```

运行基准测试：

```bash
go test -bench=. -benchmem ./utils/retry/
```

## API 参考

### 创建退避策略

#### New* 函数（返回错误）

```go
func NewConstant(t time.Duration) (Backoff, error)
func NewExponential(base time.Duration) (Backoff, error)
func NewFibonacci(base time.Duration) (Backoff, error)
func NewLinear(base time.Duration) (Backoff, error)
```

#### MustNew* 函数（不返回错误）

```go
func MustNewConstant(t time.Duration) Backoff
func MustNewExponential(base time.Duration) Backoff
func MustNewFibonacci(base time.Duration) Backoff
func MustNewLinear(base time.Duration) Backoff
```

**注意**: `MustNew*` 函数在参数无效时会使用默认值（1秒），适合在已知参数有效的场景使用。

### 退避策略装饰器

```go
func WithJitter(j time.Duration, next Backoff) Backoff
func WithJitterPercent(j uint64, next Backoff) Backoff
func WithMaxRetries(max uint64, next Backoff) Backoff
func WithCappedDuration(cap time.Duration, next Backoff) Backoff
func WithMaxDuration(timeout time.Duration, next Backoff) Backoff
```

### 便捷函数

```go
func DoQuick(ctx context.Context, f RetryFunc) error
func DoStandard(ctx context.Context, f RetryFunc) error
func DoAggressive(ctx context.Context, f RetryFunc) error
func DoWithExponential(ctx context.Context, maxRetries uint64, base time.Duration, f RetryFunc) error
func DoWithLinear(ctx context.Context, maxRetries uint64, base time.Duration, f RetryFunc) error
func DoWithConstant(ctx context.Context, maxRetries uint64, interval time.Duration, f RetryFunc) error
func DoWithTimeout(ctx context.Context, timeout, base time.Duration, f RetryFunc) error
func DoWithConfig(ctx context.Context, config *RetryConfig, f RetryFunc) error
```

### 高级功能

```go
func DoWithErrorCollection(ctx context.Context, backoff Backoff, f RetryFunc) *ErrorCollector
func DoWithCallback(ctx context.Context, backoff Backoff, f RetryFunc, callback RetryCallback) error
func DoWithPanicRecovery(ctx context.Context, backoff Backoff, f RetryFunc) error
```

### 错误处理

```go
func RetryableError(err error) error
func IsRetryable(err error) bool
```

## 常见问题

### Q: WithMaxRetries(3) 会尝试几次？

A: 总共4次。max=3 表示最多重试3次，加上首次尝试，总共4次。

### Q: 为什么我的重试次数不对？

A: 检查是否在多个 goroutine 之间共享了 Backoff 实例。每个独立的重试操作必须创建自己的实例。

### Q: 指数退避会溢出吗？

A: 会。第63次重试后会返回 math.MaxInt64（约292年）。建议配合 `WithCappedDuration` 使用。

### Q: 如何实现自定义退避策略？

A: 实现 `Backoff` 接口：

```go
type MyBackoff struct {
// 你的字段
}

func (b *MyBackoff) Next() (time.Duration, bool) {
// 你的逻辑
return duration, shouldStop
}
```

### Q: 可以在重试函数中使用 goroutine 吗？

A: 可以，但要确保正确处理 context 取消和资源清理。

### Q: WithMaxDuration 从什么时候开始计时？

A: 从第一次调用 `Next()` 开始计时，而不是从创建 Backoff 时开始。

## 示例代码

更多示例请参考：

- `example_test.go` - 基本用法示例
- `retry_test.go` - 单元测试
- `errors_test.go` - 高级功能测试
- `benchmark_test.go` - 性能测试

## 许可证

本包是 Apache 2.0 许可证，与 Go 标准库兼容。

