package gopher_fetch

import (
	"errors"
	"fmt"
	tp "gitee.com/swsk33/concurrent-task-pool/v2"
	"gitee.com/swsk33/gopher-notify"
	"net/http"
	"os"
	"time"
)

// ParallelGetTaskConfig 多线程下载任务的配置性质属性
type ParallelGetTaskConfig struct {
	// 文件的下载链接
	Url string `json:"url"`
	// 下载文件位置
	FilePath string `json:"filePath"`
	// 下载并发数
	Concurrent int `json:"concurrent"`
	// 分片请求时间间隔，若设为0则开始下载时所有分片同时开始请求
	ShardStartDelay time.Duration `json:"shardStartDelay"`
	// 下载进度记录文件位置
	processFile string
	// 是否从进度文件恢复的任务
	isRecover bool
}

// ParallelGetTaskStatus 多线程下载任务的状态性质属性
type ParallelGetTaskStatus struct {
	// 下载文件的总大小（字节）
	TotalSize int64 `json:"totalSize"`
	// 已下载部分的大小（字节）
	DownloadSize int64 `json:"-"`
	// 当前实际并发任务数
	ConcurrentTaskCount int `json:"-"`
	// 存放全部分片任务的列表
	ShardList []*shardTask `json:"shardList"`
}

// ParallelGetTask 多线程下载任务类
type ParallelGetTask struct {
	// 下载任务配置
	Config ParallelGetTaskConfig `json:"config"`
	// 下载任务状态
	Status ParallelGetTaskStatus `json:"status"`
	// 接收每个分片任务的下载事件变化的事件总线
	shardBroker *gopher_notify.Broker[string, int64]
	// 用户观察该下载任务状态的主题对象
	statusSubject *gopher_notify.Subject[*TaskStatus]
}

// NewParallelGetTask 构造函数，用于创建一个全新的分片下载任务
//
//   - url 下载地址
//   - filePath 下载文件的保存路径
//   - processFile 下载进度文件的保存位置，若传入空字符串""表示不记录为进度文件
//   - shardRequestDelay 分片请求时间间隔，若设为0则开始下载时所有分片同时开始请求
//   - concurrent 多线程下载并发数
func NewParallelGetTask(url, filePath, processFile string, shardRequestDelay time.Duration, concurrent int) *ParallelGetTask {
	// 创建任务对象
	return &ParallelGetTask{
		Config: ParallelGetTaskConfig{
			Url:             url,
			FilePath:        filePath,
			Concurrent:      concurrent,
			ShardStartDelay: shardRequestDelay,
			processFile:     processFile,
			isRecover:       false,
		},
		Status: ParallelGetTaskStatus{
			TotalSize:           0,
			DownloadSize:        0,
			ConcurrentTaskCount: 0,
			ShardList:           make([]*shardTask, 0),
		},
		shardBroker:   gopher_notify.NewBroker[string, int64](concurrent * 3),
		statusSubject: gopher_notify.NewSubject[*TaskStatus](GlobalConfig.StatusNotifyDuration),
	}
}

// NewDefaultParallelGetTask 创建一个并发任务对象
// 设定进度保存文件为下载文件所在目录下
//
//   - url 下载地址
//   - filePath 下载文件的保存路径
//   - concurrent 多线程下载并发数
func NewDefaultParallelGetTask(url, filePath string, concurrent int) *ParallelGetTask {
	return NewParallelGetTask(url, filePath, fmt.Sprintf("%s.process.json", filePath), 0, concurrent)
}

// NewSimpleParallelGetTask 创建一个最简并发任务对象
// 不保存进度文件
//
//   - url 下载地址
//   - filePath 下载文件的保存路径
//   - concurrent 多线程下载并发数
func NewSimpleParallelGetTask(url, filePath string, concurrent int) *ParallelGetTask {
	return NewParallelGetTask(url, filePath, "", 0, concurrent)
}

// NewParallelGetTaskFromFile 从进度记录文件读取并恢复一个多线程下载任务对象
//
// file 进度文件位置
func NewParallelGetTaskFromFile(file string) (*ParallelGetTask, error) {
	// 加载任务
	task, e := loadTaskFromJson[ParallelGetTask](file)
	if e != nil {
		return nil, e
	}
	// 恢复对应配置
	task.Config.processFile = file
	task.Config.isRecover = true
	// 恢复对应状态
	for _, shard := range task.Status.ShardList {
		task.Status.DownloadSize += shard.Status.DownloadSize
	}
	// 创建事件总线与主题对象
	task.shardBroker = gopher_notify.NewBroker[string, int64](task.Config.Concurrent * 3)
	task.statusSubject = gopher_notify.NewSubject[*TaskStatus](GlobalConfig.StatusNotifyDuration)
	for _, shard := range task.Status.ShardList {
		shard.statusPublisher = gopher_notify.NewBasePublisher[string, int64](task.shardBroker)
	}
	logger.Info("从文件%s恢复多线程下载任务！\n", file)
	return &task, nil
}

// 获取待下载文件大小
func (task *ParallelGetTask) getLength() error {
	// 发送HEAD请求，获取Length
	response, e := sendRequest(task.Config.Url, http.MethodHead, -1, -1)
	if e != nil {
		logger.ErrorLine("发送HEAD请求出错！")
		return e
	}
	// 如果Head不被允许，则切换为Get再试
	if response.StatusCode >= 300 {
		logger.Warn("无法使用HEAD请求，状态码：%d，将使用Get请求重试...\n", response.StatusCode)
		response, e = sendRequest(task.Config.Url, http.MethodGet, -1, -1)
		if e != nil {
			logger.ErrorLine("发送GET请求获取大小出错！")
			return e
		}
		// 最终直接关闭响应体，不进行读取
		defer func() {
			_ = response.Body.Close()
		}()
		// 再次检查状态码，若不正确则返回错误
		if response.StatusCode >= 300 {
			logger.Error("发送GET请求获取大小出错！状态码：%d\n", response.StatusCode)
			return errors.New(fmt.Sprintf("状态码不正确：%d", response.StatusCode))
		}
	}
	// 检查是否支持部分请求
	if response.Header.Get("Accept-Ranges") != "bytes" {
		return errors.New("该请求不支持部分获取，无法分片下载！")
	}
	// 读取并设定长度
	task.Status.TotalSize = response.ContentLength
	if task.Status.TotalSize <= 0 {
		return errors.New("无法获取目标文件大小！")
	}
	logger.Info("已获取下载文件大小：%d字节\n", task.Status.TotalSize)
	return nil
}

// 获取文件大小并分配任务
func (task *ParallelGetTask) allocateTask() {
	// 检查并发数与大小
	if int64(task.Config.Concurrent) > task.Status.TotalSize {
		logger.Warn("并发数：%d大于总大小：%d，将调整并发数为：%d\n", task.Config.Concurrent, task.Status.TotalSize, task.Status.TotalSize)
		task.Config.Concurrent = int(task.Status.TotalSize)
	}
	// 计算分片下载范围
	eachSize := task.Status.TotalSize / int64(task.Config.Concurrent)
	// 创建分片任务对象
	for i := 0; i < task.Config.Concurrent; i++ {
		task.Status.ShardList = append(task.Status.ShardList, newShardTask(
			task.Config.Url,
			i+1,
			task.Config.FilePath,
			int64(i)*eachSize,
			int64(i+1)*eachSize-1,
			task.shardBroker,
		))
	}
	// 处理末尾部分
	if task.Status.TotalSize%int64(task.Config.Concurrent) != 0 {
		task.Status.ShardList[task.Config.Concurrent-1].Config.RangeEnd = task.Status.TotalSize - 1
	}
	logger.Info("已完成分片计算！分片数：%d\n", task.Config.Concurrent)
}

// 创建一个与目标下载文件大小一样的空白的文件
func (task *ParallelGetTask) createFile() error {
	// 创建文件
	file, e := os.OpenFile(task.Config.FilePath, os.O_WRONLY|os.O_CREATE, 0755)
	defer func() {
		_ = file.Close()
	}()
	if e != nil {
		logger.ErrorLine("创建文件出错！")
		return e
	}
	// 调整文件大小
	e = file.Truncate(task.Status.TotalSize)
	if e != nil {
		logger.ErrorLine("调整文件大小出错！")
		return e
	}
	logger.InfoLine("已为下载文件预分配磁盘空间！")
	return nil
}

// 开始下载全部分片
func (task *ParallelGetTask) downloadShard() error {
	// 全局错误
	var totalError error
	// 创建并发任务池，下载分片数据
	taskPool := tp.NewTaskPool[*shardTask](task.Config.Concurrent, task.Config.ShardStartDelay, 0, task.Status.ShardList,
		// 每个分片任务下载逻辑
		func(shardTask *shardTask, pool *tp.TaskPool[*shardTask]) {
			// 如果任务已下载完成，则直接退出
			if shardTask.Status.TaskDone {
				logger.Warn("分片任务%d已下载完成，无需继续下载！\n", shardTask.Config.Order)
				return
			}
			// 发送分片请求进行下载
			e := shardTask.getShard()
			if e != nil {
				// 判断是否是可重试错误，若是则执行重试逻辑
				if errors.As(e, &retryErrorType) {
					logger.WarnLine(e.Error())
					pool.Retry(shardTask)
					return
				}
				// 否则，中断整个任务
				totalError = e
				pool.Interrupt()
			}
		},
		// 接收到停机信号处理逻辑
		func(pool *tp.TaskPool[*shardTask]) {
			totalError = errors.New("任务被中断！")
		},
		// 下载时每隔一段时间保存状态
		func(pool *tp.TaskPool[*shardTask]) {
			e := saveTaskToJson(task, task.Config.processFile)
			if e != nil {
				logger.ErrorLine("保存下载任务出错！")
				logger.ErrorLine(e.Error())
			}
			time.Sleep(500 * time.Millisecond)
		})
	// 创建订阅者，接收分片任务的下载变化事件
	task.shardBroker.Subscribe(sizeAdd, &sizeChangeSubscriber{task})
	task.shardBroker.Subscribe(shardStart, &shardStartSubscriber{task: task})
	task.shardBroker.Subscribe(shardDone, &shardDoneSubscriber{task})
	// 启动分片下载任务
	logger.InfoLine("开始执行分片下载...")
	// 启动分片下载
	taskPool.Start()
	// 完成下载，发布结束状态
	publishTaskStatus(task, true)
	if !taskPool.IsInterrupt() {
		logger.Info("文件：%s下载完成！\n", task.Config.FilePath)
	} else {
		logger.Warn("文件：%s未能完成下载！\n", task.Config.FilePath)
	}
	return totalError
}

// Run 开始执行多线程分片下载任务
func (task *ParallelGetTask) Run() error {
	// 如果是新建的任务，则执行任务分配
	if !task.Config.isRecover {
		// 获取文件长度
		e := task.getLength()
		if e != nil {
			return e
		}
		// 分配所有分片任务
		task.allocateTask()
		// 创建空白文件
		e = task.createFile()
		if e != nil {
			return e
		}
	}
	// 开始下载文件
	e := task.downloadShard()
	if e != nil {
		return e
	}
	// 删除进度文件
	e = os.Remove(task.Config.processFile)
	if e != nil {
		logger.Warn("删除进度文件：%s失败！请稍后手动删除！\n", task.Config.processFile)
		logger.ErrorLine(e.Error())
	}
	// 释放部分资源
	task.statusSubject.RemoveAll()
	task.shardBroker.Close()
	return nil
}

// CheckFile 检查文件摘要值，请在调用 Run 方法并下载完成后再调用该函数
//
//   - algorithm 摘要算法名称，支持： gopher_fetch.ChecksumMd5 gopher_fetch.ChecksumSha1 gopher_fetch.ChecksumSha256
//   - excepted 期望的摘要值，16进制字符串，不区分大小写
//
// 当下载的文件摘要值和excepted相同时，返回true
func (task *ParallelGetTask) CheckFile(algorithm, excepted string) (bool, error) {
	return computeFileChecksum(task.Config.FilePath, algorithm, excepted)
}

// SubscribeStatus 订阅该下载任务的实时下载状态
//
//   - lookup 观察者回调函数，当下载状态发生变化时，例如下载进度增加、实际并发数变化等，该函数就会被调用，其参数：
//     status 当前的下载状态对象
func (task *ParallelGetTask) SubscribeStatus(lookup func(status *TaskStatus)) {
	// 注册观察者
	task.statusSubject.Register(&parallelGetTaskObserver{
		task:              task,
		subscribeFunction: lookup,
		lastSize:          task.Status.DownloadSize,
		lastNotifyTime:    time.Now(),
	})
}