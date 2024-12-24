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
//   - path 文件路径
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
//   - content 写入的字节
//   - path 文件保存位置
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

// ComputeSpeed 计算网络速度
//
//   - size 一段时间内下载的数据大小，单位字节
//   - elapsed 下载 size 字节所经过的时间长度
//
// 返回计算得到的网速字符串形式，会自动换算单位
func ComputeSpeed(size float64, elapsed time.Duration) string {
	if elapsed < 0 {
		return ""
	}
	elapsedSecond := float64(elapsed) / float64(time.Second)
	bytePerSecond := size / elapsedSecond
	if 0 <= bytePerSecond && bytePerSecond <= 1024 {
		return fmt.Sprintf("%.2f Byte/s", bytePerSecond)
	}
	if bytePerSecond > 1024 && bytePerSecond <= math.Pow(1024, 2) {
		return fmt.Sprintf("%.2f KB/s", float64(bytePerSecond)/1024)
	}
	if bytePerSecond > 1024*1024 && bytePerSecond <= math.Pow(1024, 3) {
		return fmt.Sprintf("%.2f MB/s", bytePerSecond/math.Pow(1024, 2))
	}
	return fmt.Sprintf("%.2f GB/s", bytePerSecond/math.Pow(1024, 3))
}

// ComputeRemainTime 根据当前速度，计算剩余下载时间
//
//   - status 当前时刻的下载状态对象
//
// 返回剩余时间，单位：毫秒，若下载速度为0，则返回-1
func ComputeRemainTime(status *TaskStatus) float64 {
	if status.Speed <= 0 {
		return -1
	}
	return float64(status.TotalSize-status.DownloadSize) / status.Speed
}

// DefaultProcessLookup 默认的进度观察者回调函数，能够在控制台输出实时进度
//
//   - status 当前时刻的下载状态对象
func DefaultProcessLookup(status *TaskStatus) {
	realTimeLogger.Info("\033[2K当前下载进度：%.2f %% 实际并发数：%d 当前速度：%s 预计剩余时间：%.1f秒\n", float64(status.DownloadSize)/float64(status.TotalSize)*100, status.Concurrency, ComputeSpeed(status.Speed, time.Millisecond), ComputeRemainTime(status)/1000)
	if !status.IsShutdown {
		fmt.Print("\033[A")
	}
}