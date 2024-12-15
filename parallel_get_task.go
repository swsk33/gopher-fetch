package gopher_fetch

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	tp "gitee.com/swsk33/concurrent-task-pool/v2"
	"hash"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// 摘要算法名称常量
const (
	// ChecksumMd5 MD5摘要算法
	ChecksumMd5 = "MD5"
	// ChecksumSha1 SHA1摘要算法
	ChecksumSha1 = "SHA1"
	// ChecksumSha256 SHA256摘要算法
	ChecksumSha256 = "SHA256"
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
	DownloadSize int64 `json:"downloadSize"`
	// 当前实际并发任务数
	ConcurrentTaskCount int `json:"concurrentTaskCount"`
	// 存放全部分片任务的列表
	ShardList []*shardTask `json:"shardList"`
}

// ParallelGetTask 多线程下载任务类
type ParallelGetTask struct {
	// 下载任务配置
	Config ParallelGetTaskConfig `json:"config"`
	// 下载任务状态
	Status ParallelGetTaskStatus `json:"status"`
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
			TotalSize:    0,
			DownloadSize: 0,
			ShardList:    make([]*shardTask, 0),
		},
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

// NewParallelGetTaskFromFile 从进度记录文件读取并恢复一个多线程下载任务对象
//
// file 进度文件位置
func NewParallelGetTaskFromFile(file string) (*ParallelGetTask, error) {
	// 读取内容
	content, e := readFile(file)
	if e != nil {
		logger.ErrorLine(e.Error())
		return nil, e
	}
	// 反序列化
	var task ParallelGetTask
	e = json.Unmarshal(content, &task)
	if e != nil {
		logger.ErrorLine("反序列化任务内容出错！")
		return nil, e
	}
	// 设定记录文件位置字段
	task.Config.processFile = file
	// 标记为恢复任务
	task.Config.isRecover = true
	logger.Info("从文件%s恢复下载任务！\n", file)
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
		task.Status.ShardList = append(task.Status.ShardList, newShardTask(task.Config.Url, i+1, task.Config.FilePath, int64(i)*eachSize, int64(i+1)*eachSize-1))
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
	// 上一次下载大小
	var lastDownloadSize int64 = 0
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
					shardTask.Status.retryCount++
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
		// 下载时的实时检查逻辑，用于实时更新任务状态
		func(pool *tp.TaskPool[*shardTask]) {
			// 保存进度
			_ = task.saveProcess()
			// 更新状态
			updateRunningStatus(task)
			// 计算速度
			currentDownload := task.Status.DownloadSize - lastDownloadSize
			lastDownloadSize = task.Status.DownloadSize
			// 计算间隔（毫秒）
			interval := 300
			speedString := computeSpeed(currentDownload, interval)
			// 输出进度
			realTimeLogger.Info("\r当前并发数：%3d 速度：%s 总进度：%3.2f%%", task.Status.ConcurrentTaskCount, speedString, float32(task.Status.DownloadSize)/float32(task.Status.TotalSize)*100)
			time.Sleep(time.Duration(interval) * time.Millisecond)
		})
	// 启动分片下载任务
	logger.InfoLine("开始执行分片下载...")
	// 启动分片下载
	taskPool.Start()
	// 完成后换行输出一次
	realTimeLogger.InfoLine("")
	logger.Info("文件：%s下载完成！\n", task.Config.FilePath)
	return totalError
}

// 保存当前状态至进度文件
func (task *ParallelGetTask) saveProcess() error {
	if task.Config.processFile == "" {
		return nil
	}
	// 序列化
	content, e := json.Marshal(task)
	if e != nil {
		logger.ErrorLine("序列化任务对象出现错误！")
		return e
	}
	// 保存
	return writeFile(content, task.Config.processFile)
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
	return nil
}

// CheckFile 检查文件摘要值，请在调用 Run 方法并下载完成后再调用该函数
//
//   - algorithm 摘要算法名称，支持： gopher_fetch.ChecksumMd5 gopher_fetch.ChecksumSha1 gopher_fetch.ChecksumSha256
//   - excepted 期望的摘要值，16进制字符串，不区分大小写
//
// 当下载的文件摘要值和excepted相同时，返回true
func (task *ParallelGetTask) CheckFile(algorithm, excepted string) (bool, error) {
	// 打开文件
	file, e := os.Open(task.Config.FilePath)
	if e != nil {
		return false, e
	}
	defer func() {
		_ = file.Close()
	}()
	// 根据算法选择哈希函数
	var hashChecker hash.Hash
	switch algorithm {
	case ChecksumMd5:
		hashChecker = md5.New()
	case ChecksumSha1:
		hashChecker = sha1.New()
	case ChecksumSha256:
		hashChecker = sha256.New()
	default:
		return false, fmt.Errorf("不支持的摘要算法：%s", algorithm)
	}
	// 计算摘要
	_, e = io.Copy(hashChecker, file)
	if e != nil {
		logger.ErrorLine("计算文件摘要出错！")
		return false, e
	}
	// 对比
	fileHash := strings.ToLower(fmt.Sprintf("%x", hashChecker.Sum(nil)))
	exceptedLower := strings.ToLower(excepted)
	logger.InfoLine("计算摘要完成！")
	logger.Info("期望：%s\n", exceptedLower)
	logger.Info("实际：%s\n", fileHash)
	return fileHash == exceptedLower, nil
}