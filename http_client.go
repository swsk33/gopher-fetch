package gopher_fetch

import (
	"net/http"
	"net/url"
)

// 全局http请求客户端
var httpClient = &http.Client{
	Timeout: 0,
	Transport: &http.Transport{
		DisableKeepAlives: true,
	},
}

// ConfigSetProxy 设定下载代理服务器
//
// proxyUrl 代理服务器地址，例如：http://127.0.0.1:2345
func ConfigSetProxy(proxyUrl string) {
	proxy, e := url.Parse(proxyUrl)
	if e != nil {
		logger.Error("不支持的代理地址格式：%s\n", proxyUrl)
		return
	}
	httpClient.Transport.(*http.Transport).Proxy = http.ProxyURL(proxy)
	logger.Warn("将使用代理服务器：%s 进行下载\n", proxyUrl)
}

// ConfigEnvironmentProxy 配置从环境变量自动获取代理服务器配置
func ConfigEnvironmentProxy() {
	httpClient.Transport.(*http.Transport).Proxy = http.ProxyFromEnvironment
	logger.WarnLine("将从环境变量获取代理配置")
}

// ConfigDisableProxy 关闭代理配置
func ConfigDisableProxy() {
	httpClient.Transport.(*http.Transport).Proxy = nil
	logger.WarnLine("将不使用代理进行下载")
}