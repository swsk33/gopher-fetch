package gopher_fetch

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// MonoGetTaskConfig 单线程下载任务对象的配置部分
type MonoGetTaskConfig struct {
	// 文件的下载链接
	Url string `json:"url"`
	// 下载文件位置
	FilePath string `json:"filePath"`
	// 下载进度记录文件位置
	processFile string
	// 是否从进度文件恢复的任务
	isRecover bool
}

// MonoGetTaskStatus 单线程下载任务对象的状态部分
type MonoGetTaskStatus struct {
	// 下载文件的总大小（字节）
	TotalSize int64 `json:"totalSize"`
	// 已下载部分的大小（字节）
	DownloadSize int64 `json:"downloadSize"`
	// 任务重试次数
	retryCount int
}

// MonoGetTask 单线程下载任务对象
type MonoGetTask struct {
	// 配置部分
	Config MonoGetTaskConfig `json:"config"`
	// 状态部分
	Status MonoGetTaskStatus `json:"status"`
}

// NewMonoGetTask 构造函数，用于创建单线程下载任务对象
//
//   - url 下载地址
//   - filePath 下载文件的保存路径
//   - processFile 下载进度文件的保存位置，若传入空字符串""表示不记录为进度文件
func NewMonoGetTask(url, filePath, processFile string) *MonoGetTask {
	return &MonoGetTask{
		Config: MonoGetTaskConfig{
			Url:         url,
			FilePath:    filePath,
			processFile: processFile,
			isRecover:   false,
		},
		Status: MonoGetTaskStatus{
			TotalSize:    0,
			DownloadSize: 0,
			retryCount:   0,
		},
	}
}

// NewDefaultMonoGetTask 创建一个默认的单线程下载任务对象
// 设定进度保存文件为下载文件所在目录下
//
//   - url 下载地址
//   - filePath 下载文件的保存路径
func NewDefaultMonoGetTask(url, filePath string) *MonoGetTask {
	return NewMonoGetTask(url, filePath, fmt.Sprintf("%s.process.json", filePath))
}

// NewSimpleMonoGetTask 创建一个简单的单线程下载任务对象
// 不保存进度文件
//
//   - url 下载地址
//   - filePath 下载文件的保存路径
func NewSimpleMonoGetTask(url, filePath string) *MonoGetTask {
	return NewMonoGetTask(url, filePath, "")
}

// NewMonoGetTaskFromFile 从进度文件恢复单线程下载任务
//
//   - file 进度文件位置
func NewMonoGetTaskFromFile(file string) (*MonoGetTask, error) {
	// 加载任务
	task, e := loadTaskFromJson[MonoGetTask](file)
	if e != nil {
		return nil, e
	}
	// 设定对应字段
	task.Config.processFile = file
	task.Config.isRecover = true
	logger.Info("从文件%s恢复单线程下载任务！\n", file)
	return &task, nil
}

// 单线程任务重试逻辑
//
//   - reason 重试原因
//   - e 实际发生的错误
//
// 若未达到最大重试次数，则返回可重试错误对象，否则返回实际错误对象
func (task *MonoGetTask) retry(reason string, e error) error {
	// 未到最大重试次数，返回重试错误
	if task.Status.retryCount < GlobalConfig.Retry {
		task.Status.retryCount++
		return createMonoRetryError(task, reason)
	}
	// 否则，中断并返回错误
	return e
}

// 获取下载文件大小
func (task *MonoGetTask) getLength() error {
	length, supportRange, e := getContentLength(task.Config.Url)
	if e != nil {
		return e
	}
	// 不支持断点续传，则重设下载起始位置
	if !supportRange {
		task.Status.DownloadSize = 0
		logger.Warn("下载任务：%s 不支持断点续传！\n", task.Config.Url)
	}
	// 检查恢复的任务总大小是否和获取的一致
	if task.Config.isRecover && task.Status.TotalSize != length {
		return fmt.Errorf("恢复任务文件大小和请求大小不一致！(%d != %d)，请删除进度文件和已下载文件，重新创建任务！", task.Status.TotalSize, length)
	}
	// 设定总大小
	task.Status.TotalSize = length
	return nil
}

// 创建空白文件
func (task *MonoGetTask) createFile() error {
	return createBlankFile(task.Config.FilePath, task.Status.TotalSize)
}

// 发送下载请求
func (task *MonoGetTask) fetchFile() error {
	// 打开文件
	file, e := os.OpenFile(task.Config.FilePath, os.O_WRONLY, 0755)
	if e != nil {
		logger.Error("单线程任务打开文件%s失败！\n", task.Config.FilePath)
		return e
	}
	defer func() {
		_ = file.Close()
	}()
	// 设定文件起始读取位置
	_, e = file.Seek(task.Status.DownloadSize, io.SeekStart)
	if e != nil {
		logger.ErrorLine("单线程下载任务设定文件指针失败！")
		return e
	}
	// 发送请求
	var rangeStart int64 = -1
	if task.Status.DownloadSize > 0 {
		rangeStart = task.Status.DownloadSize
	}
	response, e := sendRequest(task.Config.Url, http.MethodGet, rangeStart, -1)
	if e != nil {
		// 视情况重试
		return task.retry("发送下载请求失败！", e)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	// 判断错误码
	if response.StatusCode >= 300 {
		// 重试
		return task.retry(fmt.Sprintf("发送下载请求失败！状态码不正确：%d", response.StatusCode), errors.New(fmt.Sprintf("状态码错误：%d", response.StatusCode)))
	}
	// 读取响应体
	buffer := make([]byte, 8192)
	writer := bufio.NewWriter(file)
	for {
		// 读取一次响应体
		readSize, readError := response.Body.Read(buffer)
		if readError != nil {
			// 读取完成则退出
			if readError == io.EOF {
				break
			}
			// 否则重试
			return task.retry("读取响应体错误！", readError)
		}
		// 写入文件
		_, writeError := writer.Write(buffer[:readSize])
		if writeError != nil {
			logger.ErrorLine("单线程下载任务写入文件出错！")
			return writeError
		}
		// 刷新缓冲区
		writeError = writer.Flush()
		if writeError != nil {
			logger.ErrorLine("单线程下载任务刷新文件缓冲区出错！")
			return writeError
		}
		// 记录下载进度
		task.Status.DownloadSize += int64(readSize)
	}
	logger.Info("文件%s下载完成！\n", task.Config.FilePath)
	return nil
}

// Run 启动单线程下载任务
func (task *MonoGetTask) Run() error {
	// 获取文件大小
	e := task.getLength()
	if e != nil {
		return e
	}
	// 如果不是恢复的任务，则创建空白文件
	if !task.Config.isRecover {
		e = task.createFile()
		if e != nil {
			return e
		}
	}
	// 下载文件，失败视情况重试
	for {
		e = task.fetchFile()
		if e != nil {
			// 如果是可重试错误则重试
			if errors.As(e, &retryErrorType) {
				logger.ErrorLine(e.Error())
				continue
			}
			// 否则返回错误
			return e
		}
		break
	}
	// 删除进度文件
	if task.Config.processFile != "" {
		e = os.Remove(task.Config.processFile)
		if e != nil {
			logger.Warn("删除进度文件：%s失败！请稍后手动删除！\n", task.Config.processFile)
			logger.ErrorLine(e.Error())
		}
	}
	return nil
}