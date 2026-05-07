package main

import (
	"fmt"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"sync/atomic"
	"time"
)

var GlobalXClient *XClient

var (
	RecvCount uint64
	forwCount uint64
	failCount uint64
)

// PrintStat 定时打印统计
func PrintStat() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		fmt.Printf("[xhttp统计] 接收:%d 转发成功:%d 转发失败:%d\n",
			atomic.LoadUint64(&RecvCount),
			atomic.LoadUint64(&forwCount),
			atomic.LoadUint64(&failCount))

	}
}

func ForwardHandler(path string, body []byte) {
	err := GlobalXClient.Post(path, body)
	if err != nil {
		atomic.AddUint64(&failCount, 1)
		hlog.Infof("[xhttp转发失败] %v\n", err)
		return
	}
	atomic.AddUint64(&forwCount, 1)
}
