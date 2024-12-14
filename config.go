package gopher_fetch

// FetchConfig 全局下载配置
type FetchConfig struct {
	// 最大重试次数
	Retry int
	// 请求头的UserAgent
	UserAgent string
	// 发送下载请求时，自定义的附加请求头
	Headers map[string]string
}

// GlobalConfig 全局下载配置对象
var GlobalConfig = &FetchConfig{
	Retry:     5,
	UserAgent: "GopherFetch/1.3.0",
	Headers:   make(map[string]string),
}