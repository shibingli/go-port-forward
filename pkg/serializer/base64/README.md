# Base64 编解码包 | Base64 Encoding/Decoding Package

统一的Base64编解码接口，支持通过build tags切换不同的实现。

Unified Base64 encoding/decoding interface with support for switching implementations via build tags.

## 特性 | Features

- **统一接口**：提供一致的API，无论使用哪种实现
- **灵活切换**：通过build tags在编译时选择实现
- **高性能可选**：支持使用cloudwego/base64x获得更高性能
- **完全兼容**：默认使用标准库，保证最大兼容性

- **Unified Interface**: Consistent API regardless of implementation
- **Flexible Switching**: Choose implementation at compile time via build tags
- **Optional High Performance**: Support for cloudwego/base64x for better performance
- **Full Compatibility**: Uses standard library by default for maximum compatibility

## 实现方式 | Implementations

### 1. 标准库实现（默认）| Standard Library (Default)

使用Go标准库的`encoding/base64`包。

Uses Go's standard `encoding/base64` package.

**优点 | Advantages:**

- 完全兼容标准库
- 支持流式编解码（NewEncoder/NewDecoder）
- 无额外依赖

- Fully compatible with standard library
- Supports streaming encoding/decoding (NewEncoder/NewDecoder)
- No additional dependencies

**编译命令 | Build Command:**

```bash
go build
# 或明确指定 | or explicitly specify
go build -tags '!base64x'
```

### 2. Base64x实现（高性能）| Base64x Implementation (High Performance)

使用`github.com/cloudwego/base64x`包，性能更高。

Uses `github.com/cloudwego/base64x` package for better performance.

**优点 | Advantages:**

- 性能显著提升（比标准库快2-3倍）
- 优化的SIMD实现
- 内存使用更高效

- Significantly better performance (2-3x faster than standard library)
- Optimized SIMD implementation
- More efficient memory usage

**限制 | Limitations:**

- 流式编解码通过缓冲实现（性能略低于直接调用）
- 需要额外依赖

- Streaming encoding/decoding implemented via buffering (slightly lower performance than direct calls)
- Requires additional dependency

**编译命令 | Build Command:**

```bash
go build -tags base64x
```

## 使用示例 | Usage Examples

### 基本编解码 | Basic Encoding/Decoding

```go
import "go-port-forward/pkg/serializer/base64"

// 编码 | Encode
data := []byte("Hello, World!")
encoded := base64.StdEncoding.EncodeToString(data)
fmt.Println(encoded) // SGVsbG8sIFdvcmxkIQ==

// 解码 | Decode
decoded, err := base64.StdEncoding.DecodeString(encoded)
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(decoded)) // Hello, World!
```

### URL安全编码 | URL-Safe Encoding

```go
// URL安全编码（替换+和/为-和_）
// URL-safe encoding (replaces + and / with - and _)
encoded := base64.URLEncoding.EncodeToString(data)

// 无填充的URL安全编码
// URL-safe encoding without padding
encoded := base64.RawURLEncoding.EncodeToString(data)
```

### 流式编码 | Streaming Encoding

```go
import (
    "os"
    "go-port-forward/pkg/serializer/base64"
)

// 两种实现都支持流式编码
// Both implementations support streaming encoding

file, _ := os.Create("output.txt")
defer file.Close()

encoder, err := base64.NewEncoder(base64.StdEncoding, file)
if err != nil {
    log.Fatal(err)
}
encoder.Write([]byte("Hello, World!"))
encoder.Close()
```

**实现说明 | Implementation Notes:**

- 标准库实现：直接使用 `encoding/base64.NewEncoder`，真正的流式编码
- Base64x实现：通过内部缓冲实现流式接口，在Close时一次性编码
- 使用对象池和字节池优化内存分配，减少GC压力

- Standard library: Uses `encoding/base64.NewEncoder` directly, true streaming
- Base64x: Implements streaming interface via internal buffering, encodes on Close
- Uses object pool and byte pool to optimize memory allocation and reduce GC pressure

## 性能对比 | Performance Comparison

### 直接编解码性能 | Direct Encoding/Decoding Performance

| 操作 Operation | 标准库 Std | Base64x | 提升 Improvement |
|--------------|---------|---------|----------------|
| 编码 Encode    | 100%    | ~300%   | 3x faster      |
| 解码 Decode    | 100%    | ~250%   | 2.5x faster    |

### 流式编解码性能（使用对象池和字节池优化）| Streaming Performance (with pool optimization)

**编码性能 | Encoding Performance:**

| 数据大小 | Base64x (ns/op) | 标准库 (ns/op) | 内存分配 Base64x | 内存分配 标准库     |
|------|-----------------|-------------|--------------|--------------|
| 小数据  | 177             | 341         | 160 B (4次)   | 1280 B (4次)  |
| 中等数据 | 3,552           | 7,206       | 19 KB (4次)   | 16 KB (7次)   |
| 大数据  | 41,508          | 61,978      | 254 KB (7次)  | 131 KB (10次) |

**解码性能 | Decoding Performance:**

| 数据大小 | Base64x (ns/op) | 标准库 (ns/op) | 内存分配 Base64x | 内存分配 标准库     |
|------|-----------------|-------------|--------------|--------------|
| 小数据  | 612             | 932         | 1.7 KB (8次)  | 3.7 KB (6次)  |
| 中等数据 | 8,305           | 12,448      | 31 KB (16次)  | 18 KB (9次)   |
| 大数据  | 64,462          | 106,538     | 281 KB (56次) | 133 KB (12次) |

**性能总结 | Performance Summary:**

- Base64x流式编码：比标准库快约2倍
- Base64x流式解码：比标准库快约1.5倍
- 对象池和字节池优化显著减少了小数据的内存分配

- Base64x streaming encoding: ~2x faster than stdlib
- Base64x streaming decoding: ~1.5x faster than stdlib
- Pool optimization significantly reduces memory allocation for small data

## 选择建议 | Recommendations

**使用标准库实现（默认）当：**

- 追求最大兼容性
- 处理超大文件（>100MB）需要真正的流式处理

**Use standard library (default) when:**

- Maximum compatibility is required
- Processing very large files (>100MB) requiring true streaming

**使用Base64x实现当：**

- 需要最高性能（编解码速度提升2-3倍）
- 需要流式编解码且对性能有要求
- 可以接受额外依赖

**Use Base64x when:**

- Need highest performance (2-3x faster encoding/decoding)
- Need streaming with performance requirements
- Can accept additional dependency

## 注意事项 | Notes

1. 两种实现的API完全兼容，可以无缝切换
2. Base64x的流式编解码通过缓冲实现，适合小到中等大小的数据
3. 对于超大文件（>100MB），标准库的真正流式编码性能更好
4. 使用对象池和字节池优化，显著减少内存分配和GC压力
5. Writer和Reader对象会自动归还到对象池，无需手动管理
6. pkg包下的工具代码不使用panic，而是返回错误，确保不影响宿主程序

1. Both implementations have fully compatible APIs and can be switched seamlessly
2. Base64x streaming is implemented via buffering, suitable for small to medium-sized data
3. For very large files (>100MB), standard library's true streaming has better performance
4. Uses object pool and byte pool optimization to significantly reduce memory allocation and GC pressure
5. Writer and Reader objects are automatically returned to pool, no manual management needed
6. Tools in pkg package do not use panic, but return errors to ensure host program stability

