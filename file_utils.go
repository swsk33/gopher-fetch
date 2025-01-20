package gopher_fetch

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
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
		return nil, e
	}
	defer func() {
		_ = file.Close()
	}()
	// 读取文件
	reader := bufio.NewReader(file)
	return io.ReadAll(reader)
}

// 将内容写入文件
//
//   - content 写入的字节
//   - path 文件保存位置
func writeFile(content []byte, path string) error {
	// 创建文件
	file, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if e != nil {
		return e
	}
	defer func() {
		_ = file.Close()
	}()
	// 写入文件
	writer := bufio.NewWriter(file)
	_, e = writer.Write(content)
	if e != nil {
		return e
	}
	return writer.Flush()
}

// 将下载任务序列化为JSON文件
//
//   - task 下载任务对象
//   - path 保存文件位置，若为空字符串""则不会进行任何操作
//
// 出现错误返回错误对象
func saveTaskToJson[T any](task T, path string) error {
	if path == "" {
		return nil
	}
	// 序列化
	content, e := json.Marshal(task)
	if e != nil {
		logger.ErrorLine("序列化任务对象出现错误！")
		return e
	}
	return writeFile(content, path)
}