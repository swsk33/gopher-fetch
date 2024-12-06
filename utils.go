package gopher_fetch

import (
	"bufio"
	"io"
	"os"
)

// 判断文件是否存在
//
// filePath 判断的文件或者文件夹路径
//
// 返回true说明存在
func fileExists(filePath string) bool {
	_, e := os.Stat(filePath)
	if e == nil {
		return true
	}
	return !os.IsNotExist(e)
}

// 若指定目录不存在，则创建，包括多级目录
//
// folderPath 创建的文件夹
func mkdirIfNotExists(folderPath string) error {
	if !fileExists(folderPath) {
		return os.MkdirAll(folderPath, 0755)
	}
	return nil
}

// 读取文件
//
// path 文件路径
//
// 返回文件内容的字节切片
func readFile(path string) ([]byte, error) {
	// 打开文件
	file, e := os.OpenFile(path, os.O_RDONLY, 0755)
	defer func() {
		e = file.Close()
		if e != nil {
			logger.Error("关闭文件%s时出错！\n", path)
			logger.ErrorLine(e.Error())
		}
	}()
	if e != nil {
		logger.Error("打开文件%s出错！\n", path)
		return nil, e
	}
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