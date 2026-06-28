# Go-分片下载

## 1，介绍
一个Go实现的多线程下载库，封装了多线程下载文件的逻辑，能够通过编程的方式实现基本的多线程下载操作，核心功能：

- 指定线程数的并发下载
- 支持配置例如代理、`UserAgent`、日志等
- 支持进度实时持久化和恢复
- 文件校验
- 实时下载进度的监听

## 2，安装依赖

在项目目录中执行下列命令安装依赖：

```bash
go get gitee.com/swsk33/gopher-fetch
```

## 3，进行配置

在进行多线程下载之前，我们可以进行一些配置，包括日志、代理等。

整个配置结构体定义如下：

```go
// FetchConfig 全局下载配置
type FetchConfig struct {
	// 每个分片的最大重试次数
	Retry int
	// 监听分片任务下载状态时，每次监听的间隔时间
	StatusNotifyDuration time.Duration
	// HTTP客户端配置
	HttpClient HttpClientConfig
	// 日志配置
	Log LogConfig
}

// HttpClientConfig HTTP客户端相关配置
type HttpClientConfig struct {
	// HTTP请求最大重定向次数
	MaxRedirects int
	// 使用HTTP的协议版本
	// 例如： gopher_fetch.HttpAuto, gopher_fetch.Http11 和 gopher_fetch.Http20
	HttpVersion string
	// 请求UserAgent
	UserAgent string
	// 发送下载请求时，自定义的附加请求头
	Headers map[string]string
	// 代理服务器配置
	// 例如： gopher_fetch.ProxyNone, gopher_fetch.ProxyEnv, 或者一个具体的代理服务器地址：http://127.0.0.1:1234
	Proxy string
}

// LogConfig 日志相关配置
type LogConfig struct {
	// 是否启用日志输出
	Enabled bool
	// 最低显示的日志级别
	// 例如： sclog.DEBUG, sclog.INFO, sclog.WARN, sclog.ERROR, 级别依次从低到高
	Level int
}
```

通过修改全局配置对象`gopher_fetch.GlobalConfig`的属性实现自定义配置，同时修改其属性后需要调用`gopher_fetch.ApplyGlobalConfig`使得配置生效。

默认的配置值声明如下：

```go
var GlobalConfig = &FetchConfig{
	Retry:                5,
	StatusNotifyDuration: 300 * time.Millisecond,
	HttpClient: HttpClientConfig{
		MaxRedirects: 20,
		HttpVersion:  HttpAuto,
		UserAgent:    "GopherFetch/1.9.0",
		Headers:      make(map[string]string),
		Proxy:        ProxyEnv,
	},
	Log: LogConfig{
		Enabled: true,
		Level:   sclog.INFO,
	},
}
```

通常来说，如果部分配置属性希望**维持默认**，那么就**不需要显式对这些属性赋值**，仅修改你想要自定义的属性即可。

下面，将讲解几个关键配置场景。

### (1) 日志配置

默认情况下，进行多线程下载时**会在控制台输出日志信息**，包括下载的文件信息、实时的下载进度等，可通过修改全局配置`gopher_fetch.GlobalConfig`的`Log`属性实现日志配置。

关闭控制台日志输出：

```go
// 关闭控制台日志输出
gopher_fetch.GlobalConfig.Log.Enabled = false
// 应用配置
gopher_fetch.ApplyGlobalConfig()
```

开启控制台日志输出并设定级别：

```go
// 开启控制台日志输出
gopher_fetch.GlobalConfig.Log.Enabled = true
// 设定级别为DEBUG
gopher_fetch.GlobalConfig.Log.Level = sclog.DEBUG
// 应用配置
gopher_fetch.ApplyGlobalConfig()
```

若要开启日志，建议保持默认的`INFO`级别即可，`DEBUG`级别会输出更详细的信息。

> `gopher_fetch.ApplyGlobalConfig()`在修改完所有属性后，最后**调用一次**即可。

### (2) 代理服务器

默认情况下，将会**从环境变量中读取代理服务器**并进行下载，修改全局配置`gopher_fetch.GlobalConfig`的`HttpClient.Proxy`属性实现日志配置。

从**环境变量**获取代理服务器：

```go
// 从环境变量获取代理服务器配置
gopher_fetch.GlobalConfig.HttpClient.Proxy = gopher_fetch.ProxyEnv
// 应用配置
gopher_fetch.ApplyGlobalConfig()
```

使用**自定义**的代理服务器：

```go
// 自定义代理服务器
gopher_fetch.GlobalConfig.HttpClient.Proxy = "http://127.0.0.1:1234"
// 应用配置
gopher_fetch.ApplyGlobalConfig()
```

`gopher_fetch.GlobalConfig.HttpClient.Proxy`的配置值可以是：

- `gopher_fetch.ProxyEnv` 从标准环境变量`http_proxy`和`https_proxy`读取代理服务器
- `gopher_fetch.ProxyNone` 不使用代理服务器，直接连接
- 具体的一个代理服务器地址，若地址格式错误解析失败，则会回退到不使用代理服务器

### (3) HTTP协议版本

支持配置发起下载请求时，所使用的HTTP协议版本，默认情况下使用**自动协商**策略，若要强制使用HTTP/2协议版本，配置如下：

```go
// 强制使用HTTP/2协议发起请求
gopher_fetch.GlobalConfig.HttpClient.HttpVersion = gopher_fetch.Http20
// 应用配置
gopher_fetch.ApplyGlobalConfig()
```

`gopher_fetch.GlobalConfig.HttpClient.HttpVersion`的配置值可以是：

- `gopher_fetch.HttpAuto` 自动协商HTTP协议版本，Go默认策略是尽可能使用HTTP/2版本
- `gopher_fetch.Http11` 强制使用HTTP/1.1版本
- `gopher_fetch.Http20` 强制使用HTTP/2版本

### (4) 请求头相关配置

可根据自己的需要，修改发起下载请求时的请求头相关配置：

```go
// 使用自定义UserAgent请求头为Chrome的
gopher_fetch.GlobalConfig.HttpClient.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
// 增加一些其它的自定义请求头
// GlobalConfig.Headers实际上是map[string]string类型
gopher_fetch.GlobalConfig.HttpClient.Headers["Origin"] = "example.com"
gopher_fetch.GlobalConfig.HttpClient.Headers["Authorization"] = "Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSld..."
// ...
// 应用配置
gopher_fetch.ApplyGlobalConfig()
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

- 参数`1`：`url`，下载的文件地址（文件直链地址）
- 参数`2`：`filePath`，下载文件的保存路径
- 参数`3`：`processFile`，下载进度文件的保存位置，在下载任务进行时会实时保存下载进度到进度文件，内容为JSON格式，若传入空字符串`""`表示不记录为进度文件
- 参数`4`：`shardRequestDelay`，开始进行下载任务时，分片请求时间间隔，若设为`0`则开始下载时所有分片同时开始请求
- 参数`5`：`concurrent`，多线程下载并发数

此外，还有下列构造函数可以更加简单地创建一个分片下载任务对象：

- `NewDefaultParallelGetTask(url, filePath string, concurrent int)` 创建一个并发任务对象，设定进度文件为下载文件所在目录下
- `NewSimpleParallelGetTask(url, filePath string, concurrent int)` 创建一个并发任务对象，不记录进度文件

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

## 6，文件校验

如果我们已经知道了下载的文件的摘要值（例如`MD5`、`SHA1`等等），就可以在下载完成之后对比文件摘要值，检查下载的文件是否损坏。通过`ParallelGetTask`对象的`CheckFile`方法，我们就可以在下载完成文件后进一步地校验文件：

```go
package main

import (
	"fmt"
	"gitee.com/swsk33/gopher-fetch"
)

func main() {
	// 创建一个分片下载任务
	url := "https://github.com/jgraph/drawio-desktop/releases/download/v25.0.2/draw.io-25.0.2-windows-installer.exe"
	task := gopher_fetch.NewDefaultParallelGetTask(url, "downloads/draw.io.exe", 32)
	// 运行分片下载
	e := task.Run()
	if e != nil {
		fmt.Printf("下载文件出错！%s\n", e)
		return
	}
	// 下载完成后，计算摘要，以SHA256比对为例
	// 原本文件的SHA256值
	excepted := "9a1e232896feb2218831d50c34d9b9859e0ae670efac662dc52b0ebdf7302982"
	result, e := task.CheckFile(gopher_fetch.ChecksumSha256, excepted)
	if e != nil {
		fmt.Printf("计算文件摘要出错！%s\n", e)
		return
	}
	if result {
		fmt.Println("文件未损坏！")
	} else {
		fmt.Println("文件损坏！")
	}
}
```

在执行`Run`方法完成下载之后，调用`CheckFile`方法检查已下载文件的摘要值是否和期望值匹配，该方法的参数如下：

- 参数`1`：指定摘要算法名称，可选的值有：
	- `gopher_fetch.ChecksumMd5` 使用MD5算法
	- `gopher_fetch.ChecksumSha1` 使用SHA1算法
	- `gopher_fetch.ChecksumSha256` 使用SHA256算法
- 参数`2`：期望的文件摘要，即下载的文件的原始摘要值，不区分大小写

当摘要匹配时返回`true`，说明文件未在下载过程中损坏，若计算摘要的过程中出现错误则会返回错误对象。

## 7，实时下载状态监听

### (1) 自定义监听回调函数

基于观察者模式以及发布-订阅模式，该类库还实现了用户自定义下载状态实时监听的功能，通过`ParallelGetTask`对象的`SubscribeStatus`方法，传入一个自定义的观察者回调函数，即可实现进度实时监听：

```go
package main

import (
	"fmt"
	"gitee.com/swsk33/gopher-fetch"
	"time"
)

func main() {
	// 设定实时下载状态的最短监听间隔
	gopher_fetch.GlobalConfig.StatusNotifyDuration = 500 * time.Millisecond
	// 创建一个分片下载任务
	url := "https://github.com/skeeto/w64devkit/releases/download/v2.0.0/w64devkit-x64-2.0.0.exe"
	task := gopher_fetch.NewDefaultParallelGetTask(url, "downloads/w64devkit-x64-2.0.0.exe", 32)
	// 注册监听进度回调函数
	task.SubscribeStatus(func(status *gopher_fetch.TaskStatus) {
		fmt.Printf("当前已下载：%d / %d字节，实际并发数：%d，速度：%.2f 字节/毫秒\n", status.DownloadSize, status.TotalSize, status.Concurrency, status.Speed)
	})
	// 运行分片下载
	e := task.Run()
	if e != nil {
		fmt.Printf("下载文件出错！%s\n", e)
		return
	}
}
```

上述代码在下载时，会实时输出下载的状态与速度，且每次输出间隔不小于`0.5s`。

首先，通过全局配置`gopher_fetch.GlobalConfig.StatusNotifyDuration`可指定在监听实时下载进度的时候每次监听的最短间隔，一般来说间隔不宜太短，否则会进行频繁地监听计算导致资源浪费。

然后在调用`Run`执行下载任务之前，通过`SubscribeStatus`函数注册自定义的状态监听回调函数`lookup`，在下载任务执行时每当任务状态变化（例如下载量的增加、并发数变化等）时这个观察者函数`lookup`就会被调用，并将任务的状态对象传入`lookup`函数，每次`lookup`函数被调用的时间间隔不小于`gopher_fetch.GlobalConfig.StatusNotifyDuration`定义的值。

`lookup`函数的参数如下：

- `status` 为`gopher_fetch.TaskStatus`对象指针，表示此时下载任务`ParallelGetTask`的状态

`gopher_fetch.TaskStatus`对象是表示当前时刻下载任务`ParallelGetTask`的状态的结构体，其定义如下：

```go
// TaskStatus 表示一个时刻的多线程下载任务的任务状态
type TaskStatus struct {
	// 下载文件的总大小（字节）
	TotalSize int64
	// 已下载大小（字节）
	DownloadSize int64
	// 当前实际并发数
	Concurrency int
	// 当前下载速度，单位：字节/毫秒
	Speed float64
	// 当前下载任务是否被终止或者结束
	IsShutdown bool
}
```

可在`lookup`中读取其对应属性，显示或者计算实时下载状态。

### (2) 默认提供的监听函数

除了自己实现这个`lookup`函数之外，还提供了一个内置的默认实现`gopher_fetch.DefaultProcessLookup`，能够在终端实时输出下载进度百分比，直接将这个函数传入`SubscribeStatus`即可：

```go
// 创建一个分片下载任务
task := gopher_fetch.NewDefaultParallelGetTask(url, "downloads/draw.io.exe", 32)
// 注册监听进度回调函数
// 使用默认实现
task.SubscribeStatus(gopher_fetch.DefaultProcessLookup)
e := task.Run()
// ...
```

### (3) 其它实用函数

在自己实现`lookup`函数的同时，还可以借助内置其它的一些实用函数，来完成一些计算工作：

- `gopher_fetch.ComputeSpeed(size float64, elapsed time.Duration)` 计算网络速度，返回计算得到的网速字符串形式，会自动换算单位，其参数：
	- `size` 一段时间内下载的数据大小，单位字节
	- `elapsed` 下载`size`字节所经过的时间长度，若传入负数或者`0`，则方法返回空字符串`""`
- `gopher_fetch.ComputeRemainTime(status *TaskStatus)` 根据当前任务状态以及速度，计算剩余下载时间，返回剩余时间，单位是毫秒，若下载速度为`0`则返回`-1`，其参数：
  - `status` 当前时刻的下载状态对象

可通过这些实用函数完成一些下载任务相关的计算，并在监听下载进度时显示。

## 8，单线程下载任务

多线程分片下载虽然能够在网络环境较差的情况下显著提升下载速度，但是也相对较为耗费系统资源，在多线程下载任务中维护了一个简单的工作池，以及每个分片的状态等，因此创建一个多线程分片下载任务会占用较多的CPU资源和内存资源。

因此，在网络条件较好的情况下，推荐使用单线程下载的方式，在该类库中还提供了更加轻量级的单线程下载任务对象`MonoGetTask`能够进行一个单线程下载任务，可使用其构造函数`NewMonoGetTask`创建一个`MonoGetTask`对象，构造函数参数如下：

- 参数`1`：`url`，下载地址
- 参数`2`：`filePath`，下载文件的保存路径
- 参数`3`：`processFile`，下载进度文件的保存位置，若传入空字符串`""`表示不记录为进度文件

类似地，还有下列简化的构造函数：

- `NewDefaultMonoGetTask(url, filePath string)` 创建一个默认的单线程下载任务对象，设定进度保存文件为下载文件所在目录下
- `NewSimpleMonoGetTask(url, filePath string)` 创建一个简单的单线程下载任务对象，不保存进度文件

和`ParallelGetTask`对象一样，`MonoGetTask`也支持记录进度文件，可使用函数`NewMonoGetTaskFromFile(file string)`从进度文件恢复下载任务，此外两者的方法也是相同的，对于`MonoGetTask`有下列公开方法：

- `Run()` 启动单线程下载任务
- `CheckFile(algorithm, excepted string)` 检查文件摘要值，需要在调用`Run`方法并下载完成后再调用该函数
- `SubscribeStatus(lookup func(status *TaskStatus))` 订阅该下载任务的实时下载状态

可见和`ParallelGetTask`的用法是相同的，下面给出一个单线程下载的综合示例：

```go
package main

import (
	"fmt"
	"gitee.com/swsk33/gopher-fetch"
)

func main() {
	// 创建一个单线程下载任务
	url := "https://github.com/jgraph/drawio-desktop/releases/download/v25.0.2/draw.io-25.0.2-windows-installer.exe"
	task := gopher_fetch.NewDefaultMonoGetTask(url, "downloads/draw.io.exe")
	// 使用默认的回调函数订阅下载状态
	task.SubscribeStatus(gopher_fetch.DefaultProcessLookup)
	// 运行分片下载
	e := task.Run()
	if e != nil {
		fmt.Printf("下载文件出错！%s\n", e)
		return
	}
	// 下载完成后，计算摘要，以SHA256比对为例
	// 原本文件的SHA256值
	excepted := "9a1e232896feb2218831d50c34d9b9859e0ae670efac662dc52b0ebdf7302982"
	result, e := task.CheckFile(gopher_fetch.ChecksumSha256, excepted)
	if e != nil {
		fmt.Printf("计算文件摘要出错！%s\n", e)
		return
	}
	if result {
		fmt.Println("文件未损坏！")
	} else {
		fmt.Println("文件损坏！")
	}
}
```

可根据实际情况，选择使用多线程分片下载还是单线程下载。