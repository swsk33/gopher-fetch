package gopher_fetch

import (
	"gitee.com/swsk33/sclog"
	"github.com/fatih/color"
)

// 全局日志对象
var logger = sclog.NewLogger()

// 全局实时日志对象
var realTimeLogger = sclog.NewLogger()

func init() {
	config := sclog.NewLineConfig()
	config.Time.Enabled = false
	config.Level.Enabled = false
	config.Message.Color = color.New(color.FgHiGreen)
	realTimeLogger.ConfigAll(config)
}

// ConfigEnableLogger 配置是否启用控制台日志输出
//
// enable 若为true则打开控制台日志输出，否则关闭日志
func ConfigEnableLogger(enable bool) {
	if enable {
		logger.Level = sclog.INFO
		realTimeLogger.Level = sclog.INFO
	} else {
		logger.Level = sclog.OFF
		realTimeLogger.Level = sclog.OFF
	}
}