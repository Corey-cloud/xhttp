# XHTTP

[![Go Version](https://img.shields.io/badge/Go-1.20+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

轻量高性能 **纯 TCP 单向私有协议通信框架**  

基于 Go 语言开发，专为 **网闸单向摆渡、只发不收、高并发长连接复用** 场景量身设计，轻量化替代 HTTP 实现内网高效通信。

---

# ✨ 核心特性

- 🚀 **极简私有协议**
  - 固定 4 字节大端长度头
  - `path||body` 分隔
  - 无 HTTP 冗余 Header 开销

- 🛡️ **服务端高可用**
  - 原生 TCP 监听
  - 连接限流
  - 路由自动分发
  - Panic 自动恢复，业务异常不崩服务

- 🔗 **客户端连接池**
  - 生产级长连接池
  - 最大连接数控制
  - 空闲超时回收
  - TCP 健康探测

- 📡 **完美适配网闸**
  - 纯单向通信
  - 只发不收
  - 不产生反向 RST 包
  - 适配隔离网闸单向传输

- ⏱️ **全链路超时控制**
  - 读超时
  - 写超时
  - 拨号超时
  - 空闲超时

- 📊 **内置连接池监控**
  - 实时打印连接池状态
  - 空闲连接数监控
  - 总连接数监控

- 📏 **包体大小可控**
  - 默认限制单包最大 2MB
  - 支持自定义修改

- 💻 **跨平台兼容**
  - Linux
  - Windows
  - 开箱即用

---

# 📦 协议格式

```text
[4字节大端总长度][path||body]
```

## 协议说明

| 字段 | 说明 |
|---|---|
| 前4字节 | `path\|\|body` 整体内容长度（大端） |
| path | 路由路径 |
| `\|\|` | 固定分隔符 |
| body | 原始二进制业务数据 |

---

# 🚀 安装使用

```bash
go get github.com/Corey-cloud/xhttp
```

---

# 📝 快速开始

## 服务端示例

```go
package main

import (
	"github.com/Corey-cloud/xhttp"
)

func main() {

	// 初始化路由管理器
	router := xhttp.NewXRouter()

	// 注册路由处理函数
	router.HandleFunc("/demo/test", func(path string, body []byte) {
		println("请求路由：", path)
		println("请求数据：", string(body))
	})

	// 创建服务端
	srv := xhttp.NewXServer("0.0.0.0:9999", router)

	// 启动服务
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
```

---

## 客户端示例

```go
package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Corey-cloud/xhttp"
)

func main() {
	// 初始化客户端连接池
	cli := xhttp.NewXClient(xhttp.XClientConfig{
		Addr:         "127.0.0.1:9999",
		MaxOpenConns: 100,
		MaxIdleConns: 20,
		IdleTimeout:  30 * time.Second,
		DialTimeout:  2 * time.Second,
	})

	// 开启连接池监控（可选）
	cli.StartMonitor()
	defer cli.StopMonitor()

	// 创建 ticker，每 1 秒触发一次
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 创建信号通道，监听系统中断信号（Ctrl+C 或 kill）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 计数器，用于演示
	var count int

	// 主循环
	for {
		select {
		case <-ticker.C:
			// 每 5 秒执行一次
			count++
			err := cli.Post("/demo/test", []byte("hello xhttp"))
			if err != nil {
				// 失败时打印日志（包含时间戳和计数）
				println(time.Now().Format("2006-01-02 15:04:05"),
					"发送失败 [第", count, "次]：", err.Error())
			} else {
				println(time.Now().Format("2006-01-02 15:04:05"),
					"发送成功 [第", count, "次]")
			}

		case <-sigChan:
			// 收到退出信号（Ctrl+C 或 kill）
			println("\n收到退出信号，正在优雅关闭...")
			println("总共发送次数:", count)
			return
		}
	}
}
```

---

# 🎯 适用场景

- 网闸单向数据摆渡
- 内外网隔离单向上报
- 系统日志异步上报
- 业务埋点传输
- 监控指标异步采集
- 微服务单向事件通知
- 高并发内网通信
- 长连接低延迟消息推送

---

# 💡 架构设计亮点

## 服务端设计

- 单连接独立循环读包
- 超时自动断开
- 连接之间完全隔离互不阻塞
- 路由自动分发
- 业务异步协程执行
- Panic 自动恢复保护

## 客户端设计

- 长连接池复用
- 空闲连接自动回收
- 死连接自动剔除
- 精准连接计数
- 健康探测机制

## 传输层设计

- 用户态超时读控制
- 不依赖 `SetReadDeadline`
- 规避 Windows TCP 反向 RST 问题
- 循环 Write 保证整包发送
- 彻底避免半包 / 粘包 / 丢包

---

# ⚙️ 内置常量配置

```go
const (
	pkgHeaderLen = 4                   // 固定协议头长度
	maxPkgSize   = 2 * 1024 * 1024    // 最大单包限制 2MB
)
```

---

# 📄 开源协议

本项目基于 MIT License 开源。

支持：

- 商业使用
- 二次开发
- 自由分发
- 私有修改

---

# 🤝 贡献欢迎

欢迎提交：

- Issue
- PR
- Bug 修复
- 性能优化
- 新功能扩展

一起把 XHTTP 做得更强 🚀
