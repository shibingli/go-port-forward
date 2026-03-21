// Package base64 性能基准测试 | Performance benchmark tests
package base64

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// BenchmarkStreamingEncoder 流式编码性能测试 | Streaming encoder benchmark
func BenchmarkStreamingEncoder(b *testing.B) {
	data := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100))

	b.Run("SmallData", func(b *testing.B) {
		smallData := []byte("Hello, World!")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			encoder, _ := NewEncoder(StdEncoding, &buf)
			_, _ = encoder.Write(smallData)
			_ = encoder.Close()
		}
	})

	b.Run("MediumData", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			encoder, _ := NewEncoder(StdEncoding, &buf)
			_, _ = encoder.Write(data)
			_ = encoder.Close()
		}
	})

	b.Run("LargeData", func(b *testing.B) {
		largeData := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 1000))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			encoder, _ := NewEncoder(StdEncoding, &buf)
			_, _ = encoder.Write(largeData)
			_ = encoder.Close()
		}
	})
}

// BenchmarkStreamingDecoder 流式解码性能测试 | Streaming decoder benchmark
func BenchmarkStreamingDecoder(b *testing.B) {
	data := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100))
	encoded := StdEncoding.EncodeToString(data)

	b.Run("SmallData", func(b *testing.B) {
		smallData := []byte("Hello, World!")
		smallEncoded := StdEncoding.EncodeToString(smallData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			decoder, _ := NewDecoder(StdEncoding, strings.NewReader(smallEncoded))
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, decoder)
		}
	})

	b.Run("MediumData", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			decoder, _ := NewDecoder(StdEncoding, strings.NewReader(encoded))
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, decoder)
		}
	})

	b.Run("LargeData", func(b *testing.B) {
		largeData := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 1000))
		largeEncoded := StdEncoding.EncodeToString(largeData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			decoder, _ := NewDecoder(StdEncoding, strings.NewReader(largeEncoded))
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, decoder)
		}
	})
}

// BenchmarkDirectEncoding 直接编码性能测试（对比）| Direct encoding benchmark (comparison)
func BenchmarkDirectEncoding(b *testing.B) {
	data := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100))

	b.Run("SmallData", func(b *testing.B) {
		smallData := []byte("Hello, World!")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = StdEncoding.EncodeToString(smallData)
		}
	})

	b.Run("MediumData", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = StdEncoding.EncodeToString(data)
		}
	})

	b.Run("LargeData", func(b *testing.B) {
		largeData := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 1000))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = StdEncoding.EncodeToString(largeData)
		}
	})
}

// BenchmarkDirectDecoding 直接解码性能测试（对比）| Direct decoding benchmark (comparison)
func BenchmarkDirectDecoding(b *testing.B) {
	data := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100))
	encoded := StdEncoding.EncodeToString(data)

	b.Run("SmallData", func(b *testing.B) {
		smallData := []byte("Hello, World!")
		smallEncoded := StdEncoding.EncodeToString(smallData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = StdEncoding.DecodeString(smallEncoded)
		}
	})

	b.Run("MediumData", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = StdEncoding.DecodeString(encoded)
		}
	})

	b.Run("LargeData", func(b *testing.B) {
		largeData := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 1000))
		largeEncoded := StdEncoding.EncodeToString(largeData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = StdEncoding.DecodeString(largeEncoded)
		}
	})
}
