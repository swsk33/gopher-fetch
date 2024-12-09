package gopher_fetch

import "testing"

// 测试并发下载运行
func TestParallelGetTask_Run(t *testing.T) {
	ConfigEnvironmentProxy()
	// 创建一个分片下载任务
	task, e := NewDefaultParallelGetTask("https://github.com/jgraph/drawio-desktop/releases/download/v25.0.2/draw.io-25.0.2-windows-installer.exe", "downloads/draw.io.exe", 32)
	if e != nil {
		t.Error(e)
		return
	}
	// 运行分片下载
	e = task.Run()
	if e != nil {
		t.Error(e)
		return
	}
}

// 测试从文件恢复并发任务运行
func TestParallelGetTask_Recover(t *testing.T) {
	ConfigEnvironmentProxy()
	// 从文件恢复并发任务
	task, e := NewParallelGetTaskFromFile("downloads/draw.io.exe-process.json")
	if e != nil {
		t.Error(e)
		return
	}
	e = task.Run()
	if e != nil {
		t.Error(e)
		return
	}
}