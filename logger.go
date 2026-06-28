package gopher_fetch

import (
	"sync"

	"gitee.com/swsk33/sclog"
	"github.com/fatih/color"
)

// 全局日志输出锁
var loggerLock = &sync.Mutex{}

// 全局日志对象
var logger = sclog.NewMutexLoggerShareLock(loggerLock)

// 全局实时日志对象
var realTimeLogger = sclog.NewMutexLoggerShareLock(loggerLock)

func init() {
	config := sclog.NewLineConfig()
	config.Time.Enabled = false
	config.Level.Enabled = false
	config.Message.Color = color.New(color.FgHiGreen)
	realTimeLogger.ConfigAll(config)
}

// 根据全局配置，更新日志对象
func updateLoggerConfig() {
	if GlobalConfig.Log.Enabled {
		logger.Level = GlobalConfig.Log.Level
	} else {
		logger.Level = sclog.OFF
	}
	logger.InfoLine("已更新全局日志配置")
}