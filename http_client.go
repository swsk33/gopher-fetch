package gopher_fetch

import (
	"fmt"
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

// 发送一个HTTP请求
//
//   - url 请求地址
//   - method 请求方法，例如：http.MethodHead http.MethodGet 等等
//   - rangeStart, rangeEnd 表示分片请求的范围，若不需要设定范围，则全部置为-1
func sendRequest(url, method string, rangeStart, rangeEnd int64) (*http.Response, error) {
	// 准备请求
	request, e := http.NewRequest(method, url, nil)
	if e != nil {
		logger.ErrorLine("创建请求对象出错！")
		return nil, e
	}
	// 加入请求头
	request.Header.Set("User-Agent", GlobalConfig.UserAgent)
	if rangeStart != -1 && rangeEnd != -1 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
	}
	for key, value := range GlobalConfig.Headers {
		request.Header.Set(key, value)
	}
	// 发送请求
	response, e := httpClient.Do(request)
	if e != nil {
		logger.ErrorLine("发送HTTP请求失败！")
		return nil, e
	}
	return response, nil
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