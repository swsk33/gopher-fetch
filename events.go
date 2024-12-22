package gopher_fetch

import (
	"gitee.com/swsk33/gopher-notify"
)

// 事件主题常量
const (
	// 下载数据量增加
	sizeAdd = "size-add"
	// 分片任务完成
	shardDone = "shard-done"
)

// TaskStatus 表示一个时刻的多线程下载任务的任务状态
type TaskStatus struct {
	// 已下载大小
	DownloadSize int64
	// 当前实际并发数
	Concurrency int
}

// 发布一个 ParallelGetTask 当前的状态，通知其所有的观察者
func publishTaskStatus(task *ParallelGetTask) {
	task.statusSubject.UpdateAndNotify(&TaskStatus{
		DownloadSize: task.Status.DownloadSize,
		Concurrency:  task.Status.ConcurrentTaskCount,
	}, false)
}

// 订阅下载数据量变化事件的订阅者
type sizeChangeSubscriber struct {
	// 对应的分片下载任务对象
	task *ParallelGetTask
}

// OnSubscribe 下载数据量新增时的自定义事件处理
func (subscriber *sizeChangeSubscriber) OnSubscribe(e *gopher_notify.Event[string, int64]) {
	// 改变多线程任务状态
	subscriber.task.Status.DownloadSize += e.GetData()
	// 发布多线程任务状态
	publishTaskStatus(subscriber.task)
}

// 订阅分片任务完成的订阅者
type shardDoneSubscriber struct {
	// 对应的分片下载任务对象
	task *ParallelGetTask
}

// OnSubscribe 当有一个分片任务完成时的自定义事件处理
func (subscriber *shardDoneSubscriber) OnSubscribe(e *gopher_notify.Event[string, int64]) {
	// 改变多线程任务状态
	subscriber.task.Status.ConcurrentTaskCount--
	// 发布多线程任务状态
	publishTaskStatus(subscriber.task)
}

// 观察多线程分片任务状态变化的观察者
type parallelGetTaskObserver struct {
	// 观察的分片下载任务对象
	task *ParallelGetTask
	// 用户传入的自定义接收进度变化的回调函数
	subscribeFunction func(status *TaskStatus)
}

// OnUpdate 当一个 ParallelGetTask 的下载状态发生变化时，该方法被调用
func (observer *parallelGetTaskObserver) OnUpdate(data *TaskStatus) {
	// 调用用户传入的自定义接收进度函数
	observer.subscribeFunction(data)
}