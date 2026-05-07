package main

import (
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"intelliunion_localDCRS_service/common"
)

func main() {
	common.CheckErr(common.LoadConfig())
	common.NewLogger()
	if common.Config.SendEnabled {
		GlobalXClient = NewXClient(XClientConfig{
			Addr:         common.Config.ForwardAddr,
			MaxOpenConns: 5000,
			MaxIdleConns: 200,
			IdleTimeout:  10 * time.Second,
			DialTimeout:  1 * time.Second,
		})
	}
	if common.Config.RecvEnabled {
		go func() {
			xRouter := NewXRouter()
			xRouter.HandleFunc("/demo/test", func(path string, body []byte) {
				println("请求路由：", path)
				println("请求数据：", string(body))
			})
			server := NewXServer(common.Config.RecvPort, xRouter)
			_ = server.ListenAndServe()
		}()
	}
	if common.Config.PrintStat {
		go PrintStat()
	}
	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	println("服务已停止")
}
