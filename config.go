package gopher_fetch

// FetchConfig 全局下载配置
type FetchConfig struct {
	// 最大重试次数
	Retry int
	// 请求头的UserAgent
	UserAgent string
}

// GlobalConfig 全局下载配置对象
var GlobalConfig = &FetchConfig{
	Retry:     5,
	UserAgent: "GopherFetch/1.1.2",
}