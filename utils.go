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
// timeElapsed 经过的时间长度
// 返回计算得到的网速，会自动换算单位
func computeSpeed(size int64, timeElapsed time.Duration) string {
	if timeElapsed < 0 {
		return ""
	}
	elapsedSecond := float64(timeElapsed) / float64(time.Second)
	bytePerSecond := float64(size) / elapsedSecond
	if 0 <= bytePerSecond && bytePerSecond <= 1024 {
		return fmt.Sprintf("%4.2f Byte/s", bytePerSecond)
	}
	if bytePerSecond > 1024 && bytePerSecond <= math.Pow(1024, 2) {
		return fmt.Sprintf("%6.2f KB/s", float64(bytePerSecond)/1024)
	}
	if bytePerSecond > 1024*1024 && bytePerSecond <= math.Pow(1024, 3) {
		return fmt.Sprintf("%6.2f MB/s", bytePerSecond/math.Pow(1024, 2))
	}
	return fmt.Sprintf("%6.2f GB/s", bytePerSecond/math.Pow(1024, 3))
}

// DefaultProcessLookup 默认的进度观察者回调函数，能够在控制台输出实时进度
func DefaultProcessLookup(status *TaskStatus, speedString string) {
	realTimeLogger.Info("\r当前下载进度：%6.2f%%，实际并发数：%4d，当前速度：%s", float64(status.DownloadSize)/float64(status.TotalSize)*100, status.Concurrency, speedString)
}