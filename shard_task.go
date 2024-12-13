package gopher_fetch

import (
	"bufio"
	"errors"
	"fmt"
	tp "gitee.com/swsk33/concurrent-task-pool/v2"
	"io"
	"net/http"
	"os"
)

// shardTaskConfig 一个分片下载任务的配置性质属性
type shardTaskConfig struct {
	// 下载链接
	Url string `json:"url"`
	// 分片序号，从1开始
	Order int `json:"order"`
	// 下载文件路径
	FilePath string `json:"filePath"`
	// 分片的起始范围（字节，包含）
	RangeStart int64 `json:"rangeStart"`
	// 分片的结束范围（字节，包含）
	RangeEnd int64 `json:"rangeEnd"`
}

// shardTaskStatus 一个分片下载任务的状态性质属性
type shardTaskStatus struct {
	// 已下载的部分（字节）
	DownloadSize int64 `json:"downloadSize"`
	// 该任务是否完成
	TaskDone bool `json:"taskDone"`
	// 当前分片重试次数
	retryCount int
}

// shardTask 单个分片下载任务对象
type shardTask struct {
	// 分片任务配置
	Config shardTaskConfig `json:"config"`
	// 分片任务执行状态
	Status shardTaskStatus `json:"status"`
}

// newShardTask 分片任务对象构造函数
func newShardTask(url string, order int, filePath string, rangeStart int64, rangeEnd int64) *shardTask {
	return &shardTask{
		Config: shardTaskConfig{
			Url:        url,
			Order:      order,
			FilePath:   filePath,
			RangeStart: rangeStart,
			RangeEnd:   rangeEnd,
		},
		Status: shardTaskStatus{
			DownloadSize: 0,
			TaskDone:     false,
			retryCount:   0,
		},
	}
}

// 自定义可重试的错误类型
type retryError struct {
	// 出现错误的分片编号
	order int
	// 下次重试的次数
	retryCount int
	// 错误原因
	reason string
}

// 实现error接口
func (e *retryError) Error() string {
	return e.reason
}

// 创建一个重试错误对象
func createRetryError(task *shardTask, message string) error {
	return &retryError{
		order:      task.Config.Order,
		retryCount: task.Status.retryCount + 1,
		reason:     message,
	}
}

// 任务重试逻辑
func (task *shardTask) retryShard(pool *tp.TaskPool[*shardTask], file *os.File) {
	// 修改状态
	task.Status.retryCount++
	// task.Status.DownloadSize = 0
	// 重试之前先关闭文件
	// 否则重试之前的defer操作会在重试之后关闭文件，导致文件损坏
	// _ = file.Close()
	// 执行重试
	pool.Retry(task)
	logger.Warn("将进行第%d次重试...\n", task.Status.retryCount)
}

// 下载对应分片，该方法在并发任务池中作为一个异步任务并发调用
func (task *shardTask) getShard(pool *tp.TaskPool[*shardTask]) error {
	// 打开文件
	file, e := os.OpenFile(task.Config.FilePath, os.O_WRONLY, 0755)
	if e != nil {
		logger.Error("任务%d打开文件失败！\n", task.Config.Order)
		return e
	}
	defer func() {
		_ = file.Close()
	}()
	// 计算读取位置
	startIndex := task.Config.RangeStart + task.Status.DownloadSize
	// 设定文件指针
	_, e = file.Seek(startIndex, io.SeekStart)
	if e != nil {
		logger.Error("任务%d设定文件指针失败！\n", task.Config.Order)
		return e
	}
	// 准备请求
	request, e := http.NewRequest("GET", task.Config.Url, nil)
	if e != nil {
		logger.Error("任务%d创建请求出错！\n", task.Config.Order)
		return e
	}
	// 设定请求头
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startIndex, task.Config.RangeEnd))
	request.Header.Set("User-Agent", GlobalConfig.UserAgent)
	// 发送请求
	response, e := httpClient.Do(request)
	// 出现错误则视情况重试
	if e != nil {
		// 未到最大重试次数，返回重试错误
		if task.Status.retryCount < GlobalConfig.Retry {
			return createRetryError(task, "发送下载请求失败！")
		}
		// 否则，中断并返回错误
		return e
	}
	defer func() {
		_ = response.Body.Close()
	}()
	// 判断状态码
	// 出现错误则视情况重试
	if response.StatusCode >= 300 {
		// 未到最大重试次数，返回重试错误
		if task.Status.retryCount < GlobalConfig.Retry {
			return createRetryError(task, fmt.Sprintf("发送下载请求失败！状态码不正确:%d", response.StatusCode))
		}
		// 否则，中断并返回错误
		return errors.New(fmt.Sprintf("状态码错误：%d", response.StatusCode))
	}
	// 读取请求体
	body := response.Body
	// 读取缓冲区
	buffer := make([]byte, 8092)
	// 准备写入文件
	writer := bufio.NewWriter(file)
	for {
		// 读取一次内容至缓冲区
		readSize, readError := body.Read(buffer)
		if readError != nil {
			// 如果读取完毕则退出循环
			if readError == io.EOF {
				break
			} else {
				// 视情况重试
				// 未到最大重试次数，返回重试错误
				if task.Status.retryCount < GlobalConfig.Retry {
					return createRetryError(task, "读取响应体失败！")
				}
				// 否则，中断并返回错误
				return e
			}
		}
		// 把缓冲区内容写入至文件
		_, writeError := writer.Write(buffer[:readSize])
		if writeError != nil {
			logger.Error("任务%d写入文件写入器时出现错误！\n", task.Config.Order)
			return writeError
		}
		writeError = writer.Flush()
		if writeError != nil {
			logger.Error("任务%d在写入文件时出现错误！\n", task.Config.Order)
			return writeError
		}
		// 记录下载进度
		task.Status.DownloadSize += int64(readSize)
	}
	// 标记任务完成
	task.Status.TaskDone = true
	return nil
}