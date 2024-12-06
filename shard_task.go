package gopher_fetch

// ShardTaskConfig 一个分片下载任务的配置性质属性
type ShardTaskConfig struct {
	// 下载链接
	Url string `json:"url"`
	// 分片序号，从1开始
	Order int `json:"order"`
	// 这个分片文件的路径
	ShardFilePath string `json:"shardFilePath"`
	// 分片的起始范围（字节，包含）
	RangeStart int64 `json:"rangeStart"`
	// 分片的结束范围（字节，包含）
	RangeEnd int64 `json:"rangeEnd"`
}

// ShardTaskStatus 一个分片下载任务的状态性质属性
type ShardTaskStatus struct {
	// 已下载的部分（字节）
	DownloadSize int64 `json:"downloadSize"`
	// 该任务是否完成
	TaskDone bool `json:"taskDone"`
	// 当前分片重试次数
	RetryCount int `json:"retryCount"`
}

// ShardTask 单个分片下载任务对象
type ShardTask struct {
	// 分片任务配置
	Config ShardTaskConfig `json:"config"`
	// 分片任务执行状态
	Status ShardTaskStatus `json:"status"`
}

// NewShardTask 分片任务对象构造函数
func NewShardTask(url string, order int, shardFilePath string, rangeStart int64, rangeEnd int64) *ShardTask {
	return &ShardTask{
		Config: ShardTaskConfig{
			Url:           url,
			Order:         order,
			ShardFilePath: shardFilePath,
			RangeStart:    rangeStart,
			RangeEnd:      rangeEnd,
		},
		Status: ShardTaskStatus{
			DownloadSize: 0,
			TaskDone:     false,
			RetryCount:   0,
		},
	}
}