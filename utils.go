package gopher_fetch

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

// 读取文件
//
// path 文件路径
//
// 返回文件内容的字节切片
func readFile(path string) ([]byte, error) {
	// 打开文件
	file, e := os.OpenFile(path, os.O_RDONLY, 0755)
	if e != nil {
		logger.Error("打开文件%s出错！\n", path)
		return nil, e
	}
	defer func() {
		e = file.Close()
		if e != nil {
			logger.Error("关闭文件%s时出错！\n", path)
			logger.ErrorLine(e.Error())
		}
	}()
	// 创建文件读取器
	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)
	result := make([]byte, 0)
	for {
		n, e := reader.Read(buffer)
		if e != nil {
			if e == io.EOF {
				break
			}
			logger.Error("读取文件%s时出错！\n", path)
			return nil, e
		}
		result = append(result, buffer[:n]...)
	}
	return result, nil
}

// 将内容写入文件
//
// content 写入的字节
// path 文件保存位置
func writeFile(content []byte, path string) error {
	file, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if e != nil {
		logger.Error("打开文件%s出错！\n", path)
		return e
	}
	defer func() {
		e = file.Close()
		if e != nil {
			logger.Error("关闭文件%s时出错！\n", path)
			logger.ErrorLine(e.Error())
		}
	}()
	writer := bufio.NewWriter(file)
	_, e = writer.Write(content)
	if e != nil {
		logger.Error("写入内容到文件%s时出错！\n", path)
		return e
	}
	e = writer.Flush()
	if e != nil {
		logger.ErrorLine("写入出错！")
		return e
	}
	return nil
}

// 计算网络速度
// size 一段时间内下载的数据大小，单位字节
// timeElapsed 经过的时间长度，单位毫秒
// 返回计算得到的网速，会自动换算单位
func computeSpeed(size int64, timeElapsed int) string {
	bytePerSecond := size / int64(timeElapsed) * 1000
	if 0 <= bytePerSecond && bytePerSecond <= 1024 {
		return fmt.Sprintf("%4d Byte/s", bytePerSecond)
	}
	if bytePerSecond > 1024 && bytePerSecond <= int64(math.Pow(1024, 2)) {
		return fmt.Sprintf("%6.2f KB/s", float64(bytePerSecond)/1024)
	}
	if bytePerSecond > 1024*1024 && bytePerSecond <= int64(math.Pow(1024, 3)) {
		return fmt.Sprintf("%6.2f MB/s", float64(bytePerSecond)/math.Pow(1024, 2))
	}
	return fmt.Sprintf("%6.2f GB/s", float64(bytePerSecond)/math.Pow(1024, 3))
}

// 读取一个正在执行的分片下载任务的实际并发数
//
// task 分片下载任务对象
func checkTaskConcurrentCount(task *ParallelGetTask) int {
	count := 0
	for _, eachTask := range task.Status.ShardList {
		if !eachTask.Status.TaskDone {
			count++
		}
	}
	return count
}

// 计算一个正在执行的分片下载任务全部已下载部分
func computeTaskDownloadSize(task *ParallelGetTask) int64 {
	var size int64 = 0
	for _, eachTask := range task.Status.ShardList {
		size += eachTask.Status.DownloadSize
	}
	return size
}

// 更新下载时的实时属性
func updateRunningStatus(task *ParallelGetTask) {
	// 统计当前实际并发数
	task.Status.ConcurrentTaskCount = checkTaskConcurrentCount(task)
	// 统计已下载大小
	task.Status.DownloadSize = computeTaskDownloadSize(task)
}

// 在一个单独的线程实时打印一个任务进度并更新状态
//
// task 分片下载任务对象
func printProcessAndUpdateStatus(task *ParallelGetTask) {
	go func() {
		// 上一次下载大小
		var lastDownloadSize int64 = 0
		for {
			// 保存进度
			_ = task.saveProcess()
			// 更新状态
			updateRunningStatus(task)
			// 计算速度
			currentDownload := task.Status.DownloadSize - lastDownloadSize
			lastDownloadSize = task.Status.DownloadSize
			speedString := computeSpeed(currentDownload, 300)
			// 输出进度
			realTimeLogger.Info("\r当前并发数：%3d 速度：%s 总进度：%3.2f%%", task.Status.ConcurrentTaskCount, speedString, float32(task.Status.DownloadSize)/float32(task.Status.TotalSize)*100)
			// 结束条件
			if task.Status.ConcurrentTaskCount == 0 {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}