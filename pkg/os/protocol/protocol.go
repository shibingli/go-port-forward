// Package protocol 提供协议解析和URL方案识别功能 | Package protocol provides protocol parsing and URL scheme identification
package protocol

import (
	"net/url"
	"regexp"
	"runtime"
	"strings"
)

var (
	// SchemeHttp HTTP协议方案 | HTTP protocol scheme
	SchemeHttp Scheme = "http"
	// SchemeHttps HTTPS协议方案 | HTTPS protocol scheme
	SchemeHttps Scheme = "https"
	// SchemeFile 文件协议方案 | File protocol scheme
	SchemeFile Scheme = "file"
	// SchemeUnknown 未知协议方案 | Unknown protocol scheme
	SchemeUnknown Scheme = "unknown"
)

// Scheme 协议方案类型 | Protocol scheme type
type Scheme string

// String 返回协议方案的字符串表示 | Return string representation of scheme
func (s Scheme) String() string {
	return string(s)
}

// Protocol 协议地址类型 | Protocol address type
type Protocol string

// String 返回协议地址的字符串表示 | Return string representation of protocol
func (p Protocol) String() string {
	return string(p)
}

// Value 解析协议地址，返回方案和值 | Parse protocol address, return scheme and value
func (p Protocol) Value() (scheme Scheme, value string) {
	u, err := url.Parse(p.String())
	if err != nil {
		scheme = SchemeUnknown
		return
	}

	she := strings.TrimSpace(u.Scheme)
	switch Scheme(she) {
	case SchemeHttp:
		scheme = SchemeHttp
		value = p.String()

		return
	case SchemeHttps:
		scheme = SchemeHttps
		value = p.String()

		return
	case SchemeFile:
		scheme = SchemeFile
		value = u.RequestURI()

		return
	default:
		if isWindowsPath(p.String()) {
			scheme = SchemeFile
			value = p.String()

			return
		} else if isUnixPath(p.String()) || isRelativePath(p.String()) {
			scheme = SchemeFile
			value = u.RequestURI()

			return
		}
	}

	scheme = SchemeUnknown
	value = p.String()

	return
}

func isWindowsPath(input string) bool {
	if runtime.GOOS == "windows" {
		matched, _ := regexp.MatchString(`^[a-zA-Z]:\\`, input)
		return matched
	}
	return false
}

func isUnixPath(input string) bool {
	if runtime.GOOS != "windows" {
		return strings.HasPrefix(input, "/")
	}
	return false
}

func isRelativePath(input string) bool {
	if strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") || !strings.Contains(input, ":") {
		return true
	}
	return false
}

// New 创建新的协议实例 | Create a new Protocol instance
func New(addr string) Protocol {
	addr = strings.TrimSpace(addr)
	return Protocol(addr)
}
