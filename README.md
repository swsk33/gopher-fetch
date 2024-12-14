# Go-分片下载

## 1，介绍
一个Go实现的多线程下载库，封装了多线程下载文件的逻辑，能够通过编程的方式实现基本的多线程下载操作，核心功能：

- 指定线程数的并发下载
- 支持配置例如代理、`UserAgent`等
- 支持进度实时持久化和恢复

## 2，安装依赖

在项目目录中执行下列命令安装依赖：

```bash
go get gitee.com/swsk33/gopher-fetch
```

## 3，进行配置

在进行多线程下载之前，我们可以进行一些配置，包括日志、代理等。

### (1) 日志配置

默认情况下，进行多线程下载时**会在控制台输出日志信息**，包括下载的文件信息、实时的下载进度等，可通过`ConfigEnableLogger`函数控制打开或者关闭日志控制台输出：

```go
// 关闭控制台日志输出
gopher_fetch.ConfigEnableLogger(false)
```

传入`true`打开日志，反之关闭日志。

### (2) 代理服务器

默认情况下，将会通过**直接连接**的方式进行下载，可通过下列函数配置代理服务器：

```go
// 自动从环境变量读取代理配置
// 当环境变量存在http_proxy和https_proxy时，则会读取这两个环境变量的代理服务器值，环境变量名可大写
gopher_fetch.ConfigEnvironmentProxy()
// 手动配置代理服务器，格式必须正确
gopher_fetch.ConfigSetProxy("http://127.0.0.1:1234")
// 关闭代理
gopher_fetch.ConfigDisableProxy()
```

### (3) 下载配置

通过修改全局配置结构体对象`GlobalConfig`以修改一些下载相关配置：

```go
// 使用自定义UserAgent请求头为Chrome的
gopher_fetch.GlobalConfig.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// 设定每个分片的最大重试次数为10（默认为5）
gopher_fetch.GlobalConfig.Retry = 10

// 增加一些其它的自定义请求头
// GlobalConfig.Headers实际上是map[string]string类型
gopher_fetch.GlobalConfig.Headers["Origin"] = "example.com"
gopher_fetch.GlobalConfig.Headers["Authorization"] = "Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSld..."
```

## 4，执行多线程下载

创建一个`ParallelGetTask`对象，并调用其`Run`方法即可开始多线程下载：

```go
package main

import (
	"fmt"
	"gitee.com/swsk33/gopher-fetch"
)

func main() {
	// 创建下载任务
	task := gopher_fetch.NewParallelGetTask("http://example.com/file.txt", "downloads/file.txt", "downloads/process.json", 0, 8)
	// 开始下载
	e := task.Run()
	if e != nil {
		fmt.Printf("下载失败！%s\n", e)
	}
}
```

`ParallelGetTask`表示一个多线程下载任务对象，`NewParallelGetTask`是其构造函数，该构造函数参数如下：

- 参数`1`：下载的文件地址（文件直链地址）
- 参数`2`：下载文件的保存路径
- 参数`3`：下载进度文件的保存位置，在下载任务进行时会实时保存下载进度到进度文件，内容为JSON格式，若传入空字符串`""`表示不记录为进度文件
- 参数`4`：开始进行下载任务时，分片请求时间间隔，若设为`0`则开始下载时所有分片同时开始请求
- 参数`5`：多线程下载并发数

此外，还有一个构造函数`NewDefaultParallelGetTask`可以更简单地创建多线程下载任务对象：

```go
task := gopher_fetch.NewDefaultParallelGetTask("http://example.com/file.txt", "downloads/file.txt", 8)
```

可见`NewDefaultParallelGetTask`省略了进度文件的保存位置和分片请求间隔参数，此时会将进度文件保存在下载文件的所在目录下，且设定分片请求间隔为`0`。

## 5，从进度文件恢复

如果文件下载到一半因不可抗力停止，那么我们仍然可以从保存的下载进度文件恢复，使用`NewParallelGetTaskFromFile`函数读取保存的进度文件，并恢复一个`ParallelGetTask`对象：

```go
package main

import (
	"fmt"
	"gitee.com/swsk33/gopher-fetch"
)

func main() {
	// 从保存的下载进度文件恢复一个任务对象
	task, e := gopher_fetch.NewParallelGetTaskFromFile("downloads/process.json")
	if e != nil {
		fmt.Printf("读取进度文件出错！%s\n", e)
		return
	}
	// 继续下载
	e = task.Run()
	if e != nil {
		fmt.Printf("下载失败！%s\n", e)
	}
}
```

请勿手动修改保存的下载进度文件或者删除已下载的文件部分，否则无法正确恢复下载进度！

## 6，`ParallelGetTask`的属性

`ParallelGetTask`是核心的多线程下载任务对象，它有下列公开属性：

- `Config` 为`ParallelGetTaskConfig`类型，表示下载任务的**配置内容**结构体
- `Status` 为`ParallelGetTaskStatus`类型，表示下载任务的**状态内容**结构体

上述`ParallelGetTaskConfig`类型结构体有如下公开属性：

- `Url` 文件下载链接
- `FilePath` 下载文件位置
- `Concurrent` 下载并发数
- `ShardStartDelay` 分片请求时间间隔，若设为`0`则开始下载时所有分片同时开始请求

上述`ParallelGetTaskStatus`类型结构体有如下公开属性：

- `TotalSize` 待下载文件的总大小（字节）
- `DownloadSize` 当前已下载部分的大小（字节）
- `ConcurrentTaskCount` 当前实际并发执行的任务数
- `ShardList` 全部分片任务对象列表

可以在开始下载任务后，使用另一个Goroutine中读取这些状态属性来获取实时的下载状态，或者打印配置，但尽量不要修改这些属性的值，否则可能出现错误。