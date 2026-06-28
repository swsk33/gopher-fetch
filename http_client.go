package gopher_fetch

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// 全局http请求客户端
var httpClient = &http.Client{
	// 默认配置
	Timeout: 0,
	Transport: &http.Transport{
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment,
	},
	// 重定向行为
	//  - request 即将发出的下一次请求，也就是重定向后的请求
	//  - via 已经发过的请求列表，按时间从旧到新排列
	//
	// 返回值：
	//  - 返回 nil：允许继续重定向
	//  - 返回非 nil 错误：停止重定向，并返回错误
	//  - 返回 http.ErrUseLastResponse：停止重定向，但不作为错误处理，直接返回最近一次响应，默认策略是在连续 10 次重定向后停止
	CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) > 0 {
			logger.Debug("发生重定向：%s -> %s\n",
				via[len(via)-1].URL.String(),
				request.URL.String(),
			)
		}
		if len(via) >= GlobalConfig.HttpClient.MaxRedirects {
			logger.Warn("请求：%s 已到达最大重定向次数\n", request.URL.String())
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// 根据当前全局配置值，更新当前的全局http客户端对象
func updateHttpClientConfig() {
	// 创建一个基本 Transport 对象
	transport := http.Transport{
		// 关闭复用，确保一个线程就建立一个 TCP 连接
		DisableKeepAlives: true,
	}
	// 处理代理服务器
	switch GlobalConfig.HttpClient.Proxy {
	case ProxyEnv:
		transport.Proxy = http.ProxyFromEnvironment
		logger.InfoLine("将从环境变量获取代理配置")
	case ProxyNone:
		transport.Proxy = nil
		logger.InfoLine("将不使用代理进行下载")
	default:
		proxyUrl := GlobalConfig.HttpClient.Proxy
		proxy, e := url.Parse(proxyUrl)
		if e != nil {
			logger.Error("不支持的代理地址格式：%s，将不使用代理\n", proxyUrl)
			transport.Proxy = nil
		} else {
			transport.Proxy = http.ProxyURL(proxy)
			logger.Info("将使用代理服务器：%s 进行下载\n", proxyUrl)
		}
	}
	// 处理 HTTP 协议
	switch GlobalConfig.HttpClient.HttpVersion {
	case HttpAuto:
		logger.InfoLine("HTTP协议版本将自动协商")
	case Http11:
		logger.InfoLine("将强制使用 HTTP/1.1 版本协议发起请求")
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	case Http20:
		logger.InfoLine("将强制使用 HTTP/2 版本协议发起请求")
		transport.ForceAttemptHTTP2 = true
	default:
		logger.Error("不支持的HTTP协议版本配置：%s，回退至自动协商\n", GlobalConfig.HttpClient.HttpVersion)
	}
	// 配置到 http 客户端对象
	httpClient.Transport = &transport
	logger.InfoLine("已更新全局http客户端配置")
}

// 响应读取缓冲区大小
const bufferSize = 64 * 1024

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
	request.Header.Set("User-Agent", GlobalConfig.HttpClient.UserAgent)
	if rangeStart != -1 && rangeEnd != -1 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
	} else if rangeStart != -1 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", rangeStart))
	}
	for key, value := range GlobalConfig.HttpClient.Headers {
		request.Header.Set(key, value)
	}
	// 发送请求
	response, e := httpClient.Do(request)
	if e != nil {
		logger.ErrorLine("发送HTTP请求失败！")
		return nil, e
	}
	logger.Debug("当前请求协议：%s，UserAgent：%s，最大重定向次数：%d\n", response.Proto, request.UserAgent(), GlobalConfig.HttpClient.MaxRedirects)
	return response, nil
}

// 发送 HEAD 请求探测内容大小
//
//   - url 请求地址
//
// 返回值分别是：
//   - 获取到的长度，获取失败返回-1
//   - 请求是否支持分片获取（是否支持Range请求头）
//   - 出现错误或请求失败（响应状态为4xx或5xx）则返回非空错误对象
func probeByHead(url string) (int64, bool, error) {
	// 发送HEAD请求，获取Length
	response, e := sendRequest(url, http.MethodHead, -1, -1)
	defer func() {
		_ = response.Body.Close()
	}()
	// 错误判断
	if e != nil {
		logger.ErrorLine("发送HEAD请求失败！")
		return -1, false, e
	}
	// 状态码判断
	if response.StatusCode >= 400 {
		logger.Error("HEAD请求状态码不正确：%d\n", response.StatusCode)
		return -1, false, fmt.Errorf("HEAD请求状态码错误：%d", response.StatusCode)
	}
	// 解析长度
	contentLength := response.ContentLength
	if contentLength <= 0 {
		return -1, false, errors.New("无法获取目标文件大小")
	}
	// 检查是否支持部分请求
	supportRange := response.Header.Get("Accept-Ranges") == "bytes"
	logger.Info("已获取下载文件大小：%d 字节\n", contentLength)
	// 返回
	return contentLength, supportRange, nil
}

// 发送 GET 请求探测内容大小
//
//   - url 请求地址
//
// 返回值分别是：
//   - 获取到的长度，获取失败返回-1
//   - 请求是否支持分片获取（是否支持Range请求头）
//   - 出现错误或请求失败（响应状态为4xx或5xx）则返回非空错误对象
func probeByGet(url string) (int64, bool, error) {
	// 发送GET请求，获取Length
	response, e := sendRequest(url, http.MethodGet, 0, 0)
	defer func() {
		_ = response.Body.Close()
	}()
	// 错误判断
	if e != nil {
		logger.ErrorLine("发送GET请求失败！")
		return -1, false, e
	}
	// 状态码判断
	if response.StatusCode >= 400 {
		logger.Error("GET请求状态码不正确：%d\n", response.StatusCode)
		return -1, false, fmt.Errorf("GET请求状态码错误：%d", response.StatusCode)
	}
	// 解析长度
	contentRange := response.Header.Get("Content-Range")
	if contentRange == "" || !strings.Contains(contentRange, "/") {
		return -1, false, fmt.Errorf("Content-Range为空或有误：%s", contentRange)
	}
	// 截取总大小
	totalString := strings.TrimSpace(contentRange[strings.LastIndex(contentRange, "/")+1:])
	if totalString == "*" {
		return -1, false, errors.New("无法获取请求内容长度")
	}
	totalSize, e := strconv.ParseInt(totalString, 10, 64)
	if e != nil {
		return -1, false, fmt.Errorf("解析文件大小出错：%w", e)
	}
	return totalSize, true, nil
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
	// 先发送HEAD请求
	length, support, e := probeByHead(url)
	// HEAD失败回退使用GET
	if e != nil {
		logger.WarnLine("使用HEAD获取响应体大小失败，回退使用GET请求")
		length, support, e = probeByGet(url)
		if e != nil {
			logger.Error("使用GET获取响应体大小也失败：%s\n", e)
			return -1, false, e
		}
	}
	return length, support, nil
}

// 发送下载文件请求并保存到本地
//
//   - url 下载地址
//   - filePath 保存位置（文件需已创建好）
//   - start 下载起始范围（字节），-1代表从头开始读取文件
//   - end 下载终止范围（字节），-1代表一直读取到文件尾
//   - downloadSize 记录已下载字节数的变量指针，用于任务对象维护状态
//   - fetchDone 记录文件是否完整下载完成的变量指针，用于任务对象维护状态
//   - startHook 下载开始时该回调函数会被执行，用于状态的发布-订阅逻辑，可以为nil
//   - sizeAddHook 每下载一部分文件，该回调函数就会被执行，参数表示本次下载的字节数，用于状态的发布-订阅逻辑，不能为nil
//   - doneHook 下载任务完成时，该回调函数就会被执行，用于状态的发布-订阅逻辑，不能为nil
//
// 返回值：
//   - 出现错误时，返回错误原因，否则返回空字符串""，该返回值用于重试消息提示
//   - 出现错误时返回引发错误的错误对象，否则返回nil
func downloadFile(url, filePath string, start, end int64, downloadSize *int64, fetchDone *bool, startHook func(), sizeAddHook func(addSize int64), doneHook func()) (string, error) {
	if startHook != nil {
		startHook()
	}
	// 打开文件
	file, e := os.OpenFile(filePath, os.O_WRONLY, 0755)
	if e != nil {
		return fmt.Sprintf("准备下载的文件%s失败", filePath), e
	}
	defer func() {
		_ = file.Close()
	}()
	// 设定文件起始读取位置
	if start > 0 {
		_, e = file.Seek(start, io.SeekStart)
		if e != nil {
			return "设定文件指针失败", e
		}
	}
	// 发送请求
	response, e := sendRequest(url, http.MethodGet, start, end)
	if e != nil {
		return "发送下载请求失败", e
	}
	defer func() {
		_ = response.Body.Close()
	}()
	// 判断错误码
	if response.StatusCode >= 300 {
		message := fmt.Sprintf("状态码错误：%d", response.StatusCode)
		return message, errors.New(message)
	}
	// 读取响应体
	buffer := make([]byte, bufferSize)
	// 文件写入器
	writer := bufio.NewWriter(file)
	for {
		// 读取一次响应体
		readSize, readError := response.Body.Read(buffer)
		// 处理错误，视情况重试
		if readError != nil && readError != io.EOF {
			return "读取响应体错误", readError
		}
		// 写入文件
		if readSize > 0 {
			_, writeError := writer.Write(buffer[:readSize])
			if writeError != nil {
				return "下载任务写入文件出错", writeError
			}
			// 刷新缓冲区
			writeError = writer.Flush()
			if writeError != nil {
				return "下载任务刷新文件缓冲区出错", writeError
			}
			// 记录已下载大小
			addSize := int64(readSize)
			*downloadSize += addSize
			sizeAddHook(addSize)
		}
		// 判断是否到末尾
		if readError == io.EOF {
			break
		}
	}
	// 标记任务完成
	*fetchDone = true
	doneHook()
	return "", nil
}