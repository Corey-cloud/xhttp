package main

import (
	"github.com/Corey-cloud/xhttp"
	"github.com/Corey-cloud/xhttp/common"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	common.CheckErr(common.LoadConfig())
	common.NewLogger()
	if common.Config.SendEnabled {
		xhttp.GlobalXClient = xhttp.NewXClient(xhttp.XClientConfig{
			Addr:         common.Config.ForwardAddr,
			MaxOpenConns: 5000,
			MaxIdleConns: 200,
			IdleTimeout:  10 * time.Second,
			DialTimeout:  1 * time.Second,
		})
	}
	if common.Config.RecvEnabled {
		go func() {
			xRouter := xhttp.NewXRouter()
			xRouter.HandleFunc("/demo/test", func(path string, body []byte) {
				println("请求路由：", path)
				println("请求数据：", string(body))
			})
			server := xhttp.NewXServer(common.Config.RecvPort, xRouter)
			_ = server.ListenAndServe()
		}()
	}
	if common.Config.PrintStat {
		go xhttp.PrintStat()
	}
	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	println("服务已停止")
}
