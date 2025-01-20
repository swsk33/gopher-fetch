package gopher_fetch

import "fmt"

// 自定义可重试的错误类型
type retryError struct {
	// 出现错误的分片编号
	order int
	// 下次重试的次数
	retryCount int
	// 错误原因
	reason string
}

// 表示可重试错误对象类型常量
var retryErrorType *retryError

// 实现error接口
func (e *retryError) Error() string {
	return fmt.Sprintf("分片%d出现错误！原因：%s，将进行第%d次重试...", e.order, e.reason, e.retryCount)
}

// 创建一个用于分片任务的重试错误对象
//
//   - task 分片任务对象
//   - message 重试原因
func createShardRetryError(task *shardTask, message string) error {
	return &retryError{
		order:      task.Config.Order,
		retryCount: task.Status.retryCount,
		reason:     message,
	}
}

// 创建一个用于单线程任务的重试错误对象
//
//   - task 单线程任务对象
//   - message 重试原因
func createMonoRetryError(task *MonoGetTask, message string) error {
	return &retryError{
		order:      1,
		retryCount: task.Status.retryCount,
		reason:     message,
	}
}