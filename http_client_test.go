package gopher_fetch

import (
	"testing"

	"gitee.com/swsk33/sclog"
)

// 测试获取长度
func TestGetLength(t *testing.T) {
	GlobalConfig.Log.Enabled = true
	GlobalConfig.Log.Level = sclog.DEBUG
	GlobalConfig.HttpClient.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	GlobalConfig.HttpClient.HttpVersion = Http20
	GlobalConfig.HttpClient.Proxy = ProxyEnv
	ApplyGlobalConfig()
	url := "https://github.com/ayangweb/BongoCat/releases/download/v1.1.0/BongoCat_1.1.0_amd64.deb"
	length, sup, e := getContentLength(url)
	if e != nil {
		sclog.ErrorLine(e.Error())
		t.Error(e)
	}
	sclog.Info("获取到长度：%d，是否支持分片：%v\n", length, sup)
}