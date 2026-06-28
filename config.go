package gopher_fetch

import (
	"time"

	"gitee.com/swsk33/sclog"
)

// FetchConfig 全局下载配置
type FetchConfig struct {
	// 每个分片的最大重试次数
	Retry int
	// 监听分片任务下载状态时，每次监听的间隔时间
	StatusNotifyDuration time.Duration
	// HTTP客户端配置
	HttpClient HttpClientConfig
	// 日志配置
	Log LogConfig
}

// HTTP协议版本
const (
	// HttpAuto 自动协商使用HTTP版本（默认行为）
	HttpAuto = "auto"
	// Http11 强制使用HTTP 1.1版本
	Http11 = "1.1"
	// Http20 强制使用HTTP 2.0版本
	Http20 = "2.0"
)

// 代理服务器配置常量
const (
	// ProxyNone 不使用代理
	ProxyNone = "no_proxy"
	// ProxyEnv 从环境变量读取代理（默认行为）
	ProxyEnv = "proxy_env"
)

// HttpClientConfig HTTP客户端相关配置
type HttpClientConfig struct {
	// HTTP请求最大重定向次数
	MaxRedirects int
	// 使用HTTP的协议版本
	// 例如： HttpAuto, Http11 和 Http20
	HttpVersion string
	// 请求UA
	UserAgent string
	// 发送下载请求时，自定义的附加请求头
	Headers map[string]string
	// 代理服务器配置
	// 例如： ProxyNone, ProxyEnv, 或者一个具体的代理服务器地址：http://127.0.0.1:1234
	Proxy string
}

// LogConfig 日志相关配置
type LogConfig struct {
	// 是否启用日志输出
	Enabled bool
	// 最低显示的日志级别
	// 例如： sclog.DEBUG, sclog.INFO, sclog.WARN, sclog.ERROR, 级别依次从低到高
	Level int
}

// GlobalConfig 全局下载配置对象
var GlobalConfig = &FetchConfig{
	Retry:                5,
	StatusNotifyDuration: 300 * time.Millisecond,
	HttpClient: HttpClientConfig{
		MaxRedirects: 20,
		HttpVersion:  HttpAuto,
		UserAgent:    "GopherFetch/1.9.0",
		Headers:      make(map[string]string),
		Proxy:        ProxyEnv,
	},
	Log: LogConfig{
		Enabled: true,
		Level:   sclog.INFO,
	},
}

// ApplyGlobalConfig 应用全局配置，以更新全局对象（http客户端、日志等）相关配置
func ApplyGlobalConfig() {
	updateLoggerConfig()
	updateHttpClientConfig()
}