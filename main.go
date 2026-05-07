package main

import (
	"context"
	"fmt"
	"intelliunion_localDCRS_service/action"
	"intelliunion_localDCRS_service/middleware"
	"intelliunion_localDCRS_service/resource"
	"intelliunion_localDCRS_service/router"
	"intelliunion_localDCRS_service/xhttp"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"time"

	"intelliunion_localDCRS_service/common"
	"intelliunion_localDCRS_service/service"

	"github.com/gin-gonic/gin"
)

func main() {
	os.Mkdir("runtime", os.ModeDir|os.ModePerm)
	common.DB_crypt = common.NewDBPwdCrypt() //加密解密对象
	common.CheckErr(common.LoadConfig())
	common.CheckErr(common.OpenDb())
	common.CheckErr(common.OpenRedis())
	common.SetHttpClient()
	path := common.Config.RecvLinuxPath
	switch runtime.GOOS {
	case "windows":
		path = common.Config.RecvWindowsPath
	}
	_ = os.MkdirAll(path, os.ModePerm)
	go xhttp.HandleFileCronTask()
	xhttp.NewLogger()
	if common.Config.SendEnabled {
		xhttp.GlobalXClient = xhttp.NewXClient(xhttp.XClientConfig{
			Addr:         common.Config.ForwardAddr,
			MaxOpenConns: 5000,
			MaxIdleConns: 200,
			IdleTimeout:  10 * time.Second,
			DialTimeout:  1 * time.Second,
		})
		xhttp.GlobalXClient.StartMonitor()
	}
	if common.Config.RecvEnabled {
		go func() {
			xRouter := xhttp.NewXRouter()
			// 传动链
			xRouter.HandleFunc("/api/v3/windmon/eigenvalue", action.ForwardSaveVibData)
			xRouter.HandleFunc("/api/v3/windmon/waveform", action.ForwardSaveWaveData)
			xRouter.HandleFunc("/api/v3/windmon/jx/waveform", action.ForwardWindMonJxWaveForm)
			// 塔筒
			xRouter.HandleFunc("/api/v3/tower/eigenvalue", action.ForwardSaveTowerVibData)
			// 塔筒3
			xRouter.HandleFunc("/api/v3/tower3/eigenvalue_waveform", action.ForwardSaveTower3EigenvalueWaveform)
			// 叶片
			xRouter.HandleFunc("/api/v3/blade/waveform", action.ForwardSaveBladeWaveAndVibData)
			xRouter.HandleFunc("/api/v3/blade/eigenvalue", action.ForwardSaveBladeVibData)
			// 大机组
			xRouter.HandleFunc("/api/v3/powermon/jx/waveform", action.ForwardSavePowerMonJXWaveData)
			xRouter.HandleFunc("/api/v3/powermon/vib/waveform", action.ForwardSavePowerMonVibWaveData)
			xRouter.HandleFunc("/api/v3/powermon/vib/eigenvalue", action.ForwardSavePowerMonEVVibData)
			xRouter.HandleFunc("/api/v3/powermon/pmo/eigenvalue", action.ForwardSavePowerMonEVSData)
			server := xhttp.NewXServer(common.Config.RecvPort, xRouter)
			_ = server.ListenAndServe()
		}()
	}
	if common.Config.PrintStat {
		go xhttp.PrintStat()
	}
	// logger 日志
	resource.InitResources()
	defer func() {
		resource.CloseResources()
	}()

	defer common.DB.Close()
	defer common.RedisPool.Close()

	minuteDone := make(chan int, 1)
	minuteDoneOk := make(chan int, 1)
	saveNow := make(chan int, 1)
	prepareExit := make(chan int, 1)
	exitOk := make(chan int, 1)
	saveTowerNow := make(chan int, 1)
	prepareTowerExit := make(chan int, 1)
	exitTowerOk := make(chan int, 1)
	saveTower3Now := make(chan int, 1)
	prepareTower3Exit := make(chan int, 1)
	exitTower3Ok := make(chan int, 1)
	savePMonNow := make(chan int, 1)
	preparePMonExit := make(chan int, 1)
	exitPMonOk := make(chan int, 1)
	timingNow := make(chan int, 1)
	timingPrepareExit := make(chan int, 1)
	timingExitOk := make(chan int, 1)
	timingTowerNow := make(chan int, 1)
	timingTowerPExit := make(chan int, 1)
	timingTowerExitOk := make(chan int, 1)
	timingTower3Now := make(chan int, 1)
	timingTower3PExit := make(chan int, 1)
	timingTower3ExitOk := make(chan int, 1)
	timingPMonNow := make(chan int, 1)
	timingPMonPExit := make(chan int, 1)
	timingPMonExitOk := make(chan int, 1)
	onMinute := make(chan int, 1)
	// 协程（定时任务）
	if common.Config.KeeperDataConfing.UseKeep {
		go func() {
			tick := time.NewTicker(time.Duration(common.Config.KeeperDataConfing.CronInterval) * time.Second)
			for range tick.C {
				// 将cache数据同步到MDB
				service.CacheDataToMDB()
			}
		}()
	}

	go func(done chan int) {
		tick := time.NewTicker(1 * time.Minute)
		for {
			select {
			case <-done:
				fmt.Printf("%s--->LocalDCRS minute tick goroutine will be exiting now\n",
					time.Now().Format("2006/01/02 15:04:05.999999"))
				tick.Stop()
				minuteDoneOk <- 0
				return
			case <-tick.C:
				// 一分钟同步一次
				onMinute <- 0
				service.SyncPowermonStartStopRecords()
			}
		}
	}(minuteDone)
	service.StartSaveDataAndUpdate()
	service.StartSaveTowerDataAndUpdate()
	service.StartSaveTower3DataAndUpdate()
	service.StartSavePowerMonDataAndUpdate()
	go service.TimingOrRTUpdateDeviceAlarmInfo(timingNow, timingPrepareExit, timingExitOk)
	go service.SaveAllWaveAndVibData(saveNow, prepareExit, exitOk)
	go service.TimingOrRTUpdateTowerDeviceAlarmInfo(timingTowerNow, timingTowerPExit, timingTowerExitOk)
	go service.SaveAllTowerVibData(saveTowerNow, prepareTowerExit, exitTowerOk)
	go service.TimingOrRTUpdateTower3DeviceAlarmInfo(timingTower3Now, timingTower3PExit, timingTower3ExitOk)
	go service.SaveAllTower3WaveAndEigenData(saveTower3Now, prepareTower3Exit, exitTower3Ok)
	go service.TimingOrRTUpdatePowerMonDeviceAlarmInfo(timingPMonNow, timingPMonPExit, timingPMonExitOk)
	go service.SaveAllPowerMonData(savePMonNow, preparePMonExit, exitPMonOk)
	go service.SyncLevelMeasureInfo()

	if common.Config.Mode == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	handler := router.HttpRouter()
	srv := &http.Server{
		Addr:    common.Config.Listen,
		Handler: handler,
	}

	middleware.StartLog()

	go func() {
		_ = http.ListenAndServe(":8089", nil)
	}()

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("listen: %v\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 30 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	isExit := 1
	for isExit > 0 {
		select {
		// 一分钟触发一次同步操作
		case <-onMinute:
			now := time.Now().UTC()
			saveNow <- 1
			saveTowerNow <- 1
			saveTower3Now <- 1
			savePMonNow <- 1

			//每过一段时间全量同步MySQL数据库的设备和报警门限信息到Redis
			if now.Minute()%common.HowMuchHourTimingUpdate == 0 {
				timingNow <- 1
				timingTowerNow <- 1
				timingTower3Now <- 1
				timingPMonNow <- 1
			}

			// 同步测点-层级信息
			if now.Minute()%common.HowMuchHourTimingUpdate == 0 {
				fmt.Println("同步测点-层级信息:", now.Minute())
				service.SyncLevelMeasureInfo()
				service.UpdateAllDeviceAlarmInfo()
				service.UpdateAllPowerMonDeviceAlarmInfo()
				service.SyncPowermonVibrationAndProcessOfJx()
			}
			// 删除缓存数据
			if now.Minute() == 10 {
				service.DeleteMDBCacheData(now)
			}
		case <-quit:
			fmt.Printf("%s--->LocalDCRS main goroutine get a signal to exit, will be shutdown Server...\n",
				time.Now().Format("2006/01/02 15:04:05.999999"))
			minuteDone <- 1
			<-minuteDoneOk
			timingPrepareExit <- 1
			<-timingExitOk
			prepareExit <- 1
			<-exitOk
			timingTowerPExit <- 1
			<-timingTowerExitOk
			prepareTowerExit <- 1
			<-exitTowerOk
			timingTower3PExit <- 1
			<-timingTower3ExitOk
			prepareTower3Exit <- 1
			<-exitTower3Ok
			timingPMonPExit <- 1
			<-timingPMonExitOk
			preparePMonExit <- 1
			<-exitPMonOk

			//stop http listen
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				fmt.Printf("Server Shutdown err: %v", err)
			}
			isExit = 0
		}
	}

	fmt.Printf("%s--->LocalDCRS main goroutine will exit\n", time.Now().Format("2006/01/02 15:04:05.999999"))
}
