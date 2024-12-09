package gopher_fetch

import (
	"encoding/json"
	"errors"
	"fmt"
	tp "gitee.com/swsk33/concurrent-task-pool"
	"os"
)

// ParallelGetTaskConfig 多线程下载任务的配置性质属性
type ParallelGetTaskConfig struct {
	// 文件的下载链接
	Url string `json:"url"`
	// 下载文件位置
	FilePath string `json:"filePath"`
	// 下载并发数
	Concurrent int `json:"concurrent"`
	// 下载进度记录文件位置
	processFile string
	// 是否从进度文件恢复的任务
	isRecover bool
}

// ParallelGetTaskStatus 多线程下载任务的状态性质属性
type ParallelGetTaskStatus struct {
	// 下载文件的总大小
	TotalSize int64 `json:"totalSize"`
	// 已下载部分的大小
	downloadSize int64
	// 当前并发任务数
	concurrentTaskCount int
	// 存放全部分片任务的列表
	ShardList []*ShardTask `json:"shardList"`
}

// ParallelGetTask 多线程下载任务类
type ParallelGetTask struct {
	// 下载任务配置
	Config ParallelGetTaskConfig `json:"config"`
	// 下载任务状态
	Status ParallelGetTaskStatus `json:"status"`
}

// 获取待下载文件大小
func (task *ParallelGetTask) getLength() error {
	// 获取Length
	response, e := httpClient.Head(task.Config.Url)
	if e != nil {
		logger.ErrorLine("发送HEAD请求出错！")
		return e
	}
	// 读取并设定长度
	task.Status.TotalSize = response.ContentLength
	if task.Status.TotalSize == -1 {
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
		task.Status.ShardList = append(task.Status.ShardList, NewShardTask(task.Config.Url, i+1, task.Config.FilePath, int64(i)*eachSize, int64(i+1)*eachSize-1))
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
	var totalError error
	// 创建并发任务池，下载分片数据
	taskPool := tp.NewTaskPool[*ShardTask](task.Config.Concurrent, task.Status.ShardList,
		// 每个分片任务下载逻辑
		func(shardTask *ShardTask, pool *tp.TaskPool[*ShardTask]) {
			// 如果任务已下载完成，则直接退出
			if shardTask.Status.TaskDone {
				logger.Warn("分片任务%d已下载完成，无需继续下载！\n", shardTask.Config.Order)
				return
			}
			e := shardTask.getShard(pool)
			if e != nil {
				totalError = e
				pool.Interrupt()
			}
		},
		// 接收到停机信号处理逻辑
		func(tasks []*ShardTask) {
			totalError = errors.New("任务被中断！")
		})
	// 启动分片下载任务
	logger.InfoLine("开始执行分片下载...")
	printProcess(task)
	taskPool.Start()
	// 完成后换行输出一次
	realTimeLogger.InfoLine("")
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

// NewParallelGetTask 构造函数，用于创建一个全新的分片下载任务
//
// url 下载地址
//
// filePath 下载文件的保存路径
//
// processFile 下载进度文件的保存位置，若传入空字符串""表示不记录为进度文件
//
// concurrent 多线程下载并发数
func NewParallelGetTask(url, filePath, processFile string, concurrent int) *ParallelGetTask {
	// 创建任务对象
	return &ParallelGetTask{
		Config: ParallelGetTaskConfig{
			Url:         url,
			FilePath:    filePath,
			Concurrent:  concurrent,
			processFile: processFile,
			isRecover:   false,
		},
		Status: ParallelGetTaskStatus{
			TotalSize:    0,
			downloadSize: 0,
			ShardList:    make([]*ShardTask, 0),
		},
	}
}

// NewDefaultParallelGetTask 创建一个并发任务对象，自动使用系统临时目录作为分片下载时的临时目录，并设定进度保存文件为下载文件所在目录下
//
// url 下载地址
//
// filePath 下载文件的保存路径
//
// concurrent 多线程下载并发数
func NewDefaultParallelGetTask(url, filePath string, concurrent int) *ParallelGetTask {
	return NewParallelGetTask(url, filePath, fmt.Sprintf("%s-process.json", filePath), concurrent)
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
	_ = os.Remove(task.Config.processFile)
	return nil
}