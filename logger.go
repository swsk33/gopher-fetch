package gopher_fetch

import "gitee.com/swsk33/sclog"

// 全局日志对象
var logger = sclog.NewLogger()

// ConfigEnableLogger 配置是否启用控制台日志输出
//
// enable 若为true则打开控制台日志输出，否则关闭日志
func ConfigEnableLogger(enable bool) {
	if enable {
		logger.Level = sclog.INFO
	} else {
		logger.Level = sclog.OFF
	}
}