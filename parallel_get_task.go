package gopher_fetch

import "encoding/json"

// ParallelGetTaskConfig 多线程下载任务的配置性质属性
type ParallelGetTaskConfig struct {
	// 文件的下载链接
	Url string `json:"url"`
	// 文件的最终保存位置
	FilePath string `json:"filePath"`
	// 下载并发数
	Concurrent int `json:"concurrent"`
	// 下载的分片临时文件保存文件夹（不存在会自动创建）
	TempFolder string `json:"tempFolder"`
	// 下载进度记录文件位置
	ProcessFile string `json:"processFile"`
}

// ParallelGetTaskStatus 多线程下载任务的状态性质属性
type ParallelGetTaskStatus struct {
	// 下载文件的总大小
	TotalSize int64 `json:"totalSize"`
	// 已下载部分的大小
	DownloadSize int64 `json:"downloadSize"`
	// 当前任务是否暂停
	IsPause bool `json:"isPause"`
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

// NewParallelGetTask 的构造函数
//
// url 下载地址
//
// filePath 下载文件的保存路径
//
// tempFolder 下载文件时分片的临时文件存放目录（不存在会自动创建）
//
// processFile 下载进度文件的保存位置，若传入空字符串""表示不记录为进度文件
//
// concurrent 多线程下载并发数
func NewParallelGetTask(url, filePath, tempFolder, processFile string, concurrent int) *ParallelGetTask {
	return &ParallelGetTask{
		Config: ParallelGetTaskConfig{
			Url:         url,
			FilePath:    filePath,
			Concurrent:  concurrent,
			TempFolder:  tempFolder,
			ProcessFile: processFile,
		},
		Status: ParallelGetTaskStatus{
			TotalSize:    0,
			DownloadSize: 0,
			IsPause:      false,
			ShardList:    make([]*ShardTask, 0),
		},
	}
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
	return &task, nil
}