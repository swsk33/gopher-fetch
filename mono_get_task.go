package gopher_fetch

import (
	"errors"
	"fmt"
	"gitee.com/swsk33/gopher-notify"
	"os"
	"time"
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
	// 任务是否下载完成
	taskDone bool
	// 任务重试次数
	retryCount int
}

// MonoGetTask 单线程下载任务对象
type MonoGetTask struct {
	// 配置部分
	Config MonoGetTaskConfig `json:"config"`
	// 状态部分
	Status MonoGetTaskStatus `json:"status"`
	// 用于用户监听进度事件的观察者主题
	subject *gopher_notify.Subject[*TaskStatus]
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
			taskDone:     false,
			retryCount:   0,
		},
		subject: gopher_notify.NewSubject[*TaskStatus](GlobalConfig.StatusNotifyDuration),
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
	// 创建观察者主题
	task.subject = gopher_notify.NewSubject[*TaskStatus](GlobalConfig.StatusNotifyDuration)
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
	// 下载文件
	errorMessage, e := downloadFile(task.Config.Url, task.Config.FilePath, task.Status.DownloadSize, -1, &task.Status.DownloadSize, &task.Status.taskDone,
		nil,
		func(addSize int64) {
			publishMonoTaskStatus(task, false)
		},
		func() {
			publishMonoTaskStatus(task, true)
		})
	// 出现错误视情况返回重试错误
	if e != nil {
		return task.retry(errorMessage, e)
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
	// 在新的线程中定时保存进度
	if task.Config.processFile != "" {
		go func() {
			for !task.Status.taskDone {
				// 保存下载文件
				saveProcessError := saveTaskToJson[*MonoGetTask](task, task.Config.processFile)
				if saveProcessError != nil {
					logger.ErrorLine("保存单线程任务进度文件出错！")
					logger.ErrorLine(saveProcessError.Error())
				}
				time.Sleep(350 * time.Millisecond)
			}
		}()
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
	// 释放部分资源
	task.subject.RemoveAll()
	return nil
}

// CheckFile 检查文件摘要值，请在调用 Run 方法并下载完成后再调用该函数
//
//   - algorithm 摘要算法名称，支持： gopher_fetch.ChecksumMd5 gopher_fetch.ChecksumSha1 gopher_fetch.ChecksumSha256
//   - excepted 期望的摘要值，16进制字符串，不区分大小写
//
// 当下载的文件摘要值和excepted相同时，返回true
func (task *MonoGetTask) CheckFile(algorithm, excepted string) (bool, error) {
	return computeFileChecksum(task.Config.FilePath, algorithm, excepted)
}

// SubscribeStatus 订阅该下载任务的实时下载状态
//
//   - lookup 观察者回调函数，当下载状态发生变化时，例如下载进度增加、实际并发数变化等，该函数就会被调用，其参数：
//     status 当前的下载状态对象
func (task *MonoGetTask) SubscribeStatus(lookup func(status *TaskStatus)) {
	// 注册观察者
	task.subject.Register(&taskObserver{
		subscribeFunction: lookup,
		lastSize:          task.Status.DownloadSize,
		lastNotifyTime:    time.Now(),
	})
}