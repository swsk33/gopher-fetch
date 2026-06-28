package gopher_fetch

import (
	"testing"

	"gitee.com/swsk33/sclog"
)

const (
	parallelTaskUrl    = "https://cdimage.debian.org/debian-cd/current/amd64/iso-dvd/debian-13.5.0-amd64-DVD-1.iso"
	parallelFileSha256 = "343b6e02a8bdf6429eb3722ee0056b5c7d9ad17d88328e499909da7205e55f50"
)

// 运行多线程下载任务测试
func runParallelTestTask(task *ParallelGetTask, t *testing.T) {
	// 监听分片任务的下载状态
	// 使用默认的函数
	task.SubscribeStatus(DefaultProcessLookup)
	// 运行分片下载
	e := task.Run()
	if e != nil {
		t.Error(e)
		return
	}
	// 计算摘要
	result, e := task.CheckFile(ChecksumSha256, parallelFileSha256)
	if e != nil {
		t.Error(e)
		return
	}
	if result {
		logger.InfoLine("文件未损坏！")
	} else {
		logger.ErrorLine("文件损坏！")
		t.Error("文件下载损坏！")
		t.Fail()
	}
}

// 测试并发下载运行
func TestParallelGetTask_Run(t *testing.T) {
	GlobalConfig.Log.Level = sclog.DEBUG
	ApplyGlobalConfig()
	// 创建一个分片下载任务
	task := NewDefaultParallelGetTask(parallelTaskUrl, "downloads/debian-13.5.iso", 32)
	// 执行任务
	runParallelTestTask(task, t)
}

// 测试从文件恢复并发任务运行
func TestParallelGetTask_Recover(t *testing.T) {
	GlobalConfig.Log.Level = sclog.DEBUG
	ApplyGlobalConfig()
	// 从文件恢复并发任务
	task, e := NewParallelGetTaskFromFile("downloads/debian-13.5.iso.process.json")
	if e != nil {
		t.Error(e)
		return
	}
	// 执行任务
	runParallelTestTask(task, t)
}