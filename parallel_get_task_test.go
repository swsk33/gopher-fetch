package gopher_fetch

import (
	"testing"
)

// 测试并发下载运行
func TestParallelGetTask_Run(t *testing.T) {
	ConfigEnvironmentProxy()
	// 加入自定义请求头
	GlobalConfig.Headers["Origin"] = "github.com"
	// 创建一个分片下载任务
	url := "https://github.com/jgraph/drawio-desktop/releases/download/v25.0.2/draw.io-25.0.2-windows-installer.exe"
	task := NewDefaultParallelGetTask(url, "downloads/draw.io.exe", 32)
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
	result, e := task.CheckFile(ChecksumSha256, "9a1e232896feb2218831d50c34d9b9859e0ae670efac662dc52b0ebdf7302982")
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

// 测试从文件恢复并发任务运行
func TestParallelGetTask_Recover(t *testing.T) {
	ConfigEnvironmentProxy()
	// 从文件恢复并发任务
	task, e := NewParallelGetTaskFromFile("downloads/draw.io.exe.process.json")
	if e != nil {
		t.Error(e)
		return
	}
	// 监听分片任务的下载状态
	// 使用默认的函数
	task.SubscribeStatus(DefaultProcessLookup)
	e = task.Run()
	if e != nil {
		t.Error(e)
		return
	}
	// 计算摘要
	result, e := task.CheckFile(ChecksumSha256, "9a1e232896feb2218831d50c34d9b9859e0ae670efac662dc52b0ebdf7302982")
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