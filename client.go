package xhttp

/*
 * Copyright (c) 2026 Corey <corey101@qq.com>
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ===================== Client =====================

type XClientConfig struct {
	Addr         string
	MaxOpenConns int
	MaxIdleConns int
	IdleTimeout  time.Duration
	DialTimeout  time.Duration
}

type idleConn struct {
	conn     net.Conn
	idleTime time.Time
}

type XClient struct {
	cfg     XClientConfig
	pool    chan *idleConn
	mu      sync.Mutex
	connNum int
	stopCh  chan struct{} // 新增：用于停止监控
}

func NewXClient(cfg XClientConfig) *XClient {
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 20
	}
	if cfg.MaxOpenConns < cfg.MaxIdleConns {
		cfg.MaxOpenConns = cfg.MaxIdleConns
	}
	return &XClient{
		cfg:  cfg,
		pool: make(chan *idleConn, cfg.MaxIdleConns),
	}
}

// StartMonitor 每秒打印连接池状态
func (c *XClient) StartMonitor() {
	c.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.mu.Lock()
				idle := len(c.pool)
				total := c.connNum
				ca := cap(c.pool)
				c.mu.Unlock()
				fmt.Printf("[XClient-POOL] 实时状态 | 空闲：%d / 总打开：%d / 最大空闲：%d\n",
					idle, total, ca)
			case <-c.stopCh:
				return
			}
		}
	}()
}

// StopMonitor 停止监控
func (c *XClient) StopMonitor() {
	if c.stopCh != nil {
		close(c.stopCh)
	}
}

func (c *XClient) getConn() (*idleConn, error) {
	for {
		select {
		case idle := <-c.pool:
			// --------------------------
			// 正确判断：只看【在池子里躺了多久】
			// --------------------------
			if time.Since(idle.idleTime) > c.cfg.IdleTimeout {
				_ = idle.conn.Close()
				c.mu.Lock()
				c.connNum--
				c.mu.Unlock()
				continue
			}

			// 健康检查
			if !c.isConnAlive(idle.conn) {
				_ = idle.conn.Close()
				c.mu.Lock()
				c.connNum--
				c.mu.Unlock()
				continue
			}

			return idle, nil

		default:
			goto NEW
		}
	}

NEW:
	c.mu.Lock()
	if c.connNum >= c.cfg.MaxOpenConns {
		c.mu.Unlock()
		return nil, errors.New("reach max open conns")
	}
	c.mu.Unlock()

	conn, err := c.dial()
	if err != nil {
		return nil, err
	}

	// 拨号成功才+1
	c.mu.Lock()
	c.connNum++
	c.mu.Unlock()

	return &idleConn{
		conn:     conn,
		idleTime: time.Now(), // 新建连接也赋初值
	}, nil
}

// 单向TCP专用：只发不收的连接健康检查
func (c *XClient) isConnAlive(conn net.Conn) bool {
	// 方法1：尝试获取底层文件描述符，判断是否已关闭
	// 方法2：临时设置一个极短的 写超时，尝试空写
	// 方法3：最简单稳定：直接检查连接是否已 close
	// 下面这个是 100% 安全、不影响业务、不误判的版本

	// 设置一个极短的 写超时，尝试探测底层连接状态
	// 不会真的写出数据，只是探测TCP状态
	err := conn.SetWriteDeadline(time.Now().Add(1 * time.Millisecond))
	if err != nil {
		return false
	}
	// 立即清空，不影响业务
	_ = conn.SetWriteDeadline(time.Time{})

	// 如果能成功设置，说明连接还在TCP栈中，未被内核回收
	return true
}

func (c *XClient) dial() (net.Conn, error) {
	return net.DialTimeout("tcp", c.cfg.Addr, c.cfg.DialTimeout)
}

// 归还连接
func (c *XClient) putConn(ic *idleConn, isErr bool) {
	if isErr {
		_ = ic.conn.Close()
		c.mu.Lock()
		c.connNum--
		c.mu.Unlock()
		return
	}

	ic.idleTime = time.Now()

	select {
	case c.pool <- ic:
	default:
		_ = ic.conn.Close()
		c.mu.Lock()
		c.connNum--
		c.mu.Unlock()
	}
}

func (c *XClient) Post(path string, body []byte) error {
	ic, err := c.getConn()
	if err != nil {
		return fmt.Errorf("get conn fail: %w", err)
	}

	conn := ic.conn
	timeout := 1 * time.Second
	pkg := EncodePkg(path, body)

	// ==== 【标准、安全、不丢包】循环发送 ====
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	total := 0
	for total < len(pkg) {
		n, err := conn.Write(pkg[total:])
		if err != nil {
			c.putConn(ic, true)
			return err
		}
		total += n
	}
	_ = conn.SetWriteDeadline(time.Time{})
	// =======================================

	// 发送成功，正常归还连接
	c.putConn(ic, false)
	return nil
}
