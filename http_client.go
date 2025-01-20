package gopher_fetch

import (
	"errors"
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
//   - rangeStart, rangeEnd 表示分片请求的范围，若不需要设定范围，则全部置为-1，若起始不为-1但终止为-1，则获取从起始开始往后的全部内容
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
	} else if rangeStart != -1 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", rangeStart))
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

// 获取请求的文件大小
//
//   - url 请求地址
//
// 返回值分别是：
//   - 获取到的长度，获取失败返回-1
//   - 请求是否支持分片获取（是否支持Range请求头）
//   - 出现错误则返回非空错误对象
func getContentLength(url string) (int64, bool, error) {
	// 发送HEAD请求，获取Length
	response, e := sendRequest(url, http.MethodHead, -1, -1)
	if e != nil {
		logger.ErrorLine("发送HEAD请求出错！")
		return -1, false, e
	}
	// 如果Head不被允许，则切换为Get再试
	if response.StatusCode >= 300 {
		logger.Warn("无法使用HEAD请求，状态码：%d，将使用GET请求重试...\n", response.StatusCode)
		response, e = sendRequest(url, http.MethodGet, -1, -1)
		if e != nil {
			logger.ErrorLine("发送GET请求获取大小出错！")
			return -1, false, e
		}
		// 最终直接关闭响应体，不进行读取
		defer func() {
			_ = response.Body.Close()
		}()
		// 再次检查状态码，若不正确则返回错误
		if response.StatusCode >= 300 {
			logger.Error("发送GET请求获取大小出错！状态码：%d\n", response.StatusCode)
			return -1, false, fmt.Errorf("状态码不正确：%d", response.StatusCode)
		}
	}
	// 读取长度
	if response.ContentLength <= 0 {
		return -1, false, errors.New("无法获取目标文件大小！")
	}
	// 检查是否支持部分请求
	supportRange := response.Header.Get("Accept-Ranges") == "bytes"
	logger.Info("已获取下载文件大小：%d字节\n", response.ContentLength)
	return response.ContentLength, supportRange, nil
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