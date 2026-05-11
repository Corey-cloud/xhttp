package xhttp

/*
 * Copyright (c) 2026 Corey <corey101@qq.com>
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
)

const pkgHeaderLen = 4
const maxPkgSize = 2 * 1024 * 1024 // 2MB 上限

// ===================== Router =====================

type XHandler func(path string, body []byte)

type XRouter struct {
	mu        sync.RWMutex
	routerMap map[string]XHandler
}

func NewXRouter() *XRouter {
	return &XRouter{
		routerMap: make(map[string]XHandler),
	}
}

func (r *XRouter) HandleFunc(path string, h XHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routerMap[path] = h
}

func (r *XRouter) Dispatch(path string, body []byte) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hd, ok := r.routerMap[path]
	if !ok {
		hlog.Errorf("[XHTTP] 未注册的 path: %s", path)
		return false
	}
	// ========= 业务 panic 自动捕获，不炸服务 =========
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("[XHTTP] 业务函数 panic 捕获: path=%s, err=%v\n", path, err)
			}
		}()
		hd(path, body)
	}()
	// ======================================================
	return true
}

// ===================== Protocol =====================

func EncodePkg(path string, body []byte) []byte {
	content := append([]byte(path+"||"), body...)
	totalLen := len(content)

	pkg := make([]byte, pkgHeaderLen+totalLen)
	binary.BigEndian.PutUint32(pkg[:pkgHeaderLen], uint32(totalLen))
	copy(pkg[pkgHeaderLen:], content)

	return pkg
}

func DecodePkg(data []byte) (string, []byte, error) {
	if len(data) <= pkgHeaderLen {
		return "", nil, errors.New("pkg too short")
	}

	content := data[pkgHeaderLen:]
	idx := bytes.Index(content, []byte("||"))
	if idx <= 0 {
		return "", nil, errors.New("split path err")
	}

	path := string(content[:idx])
	body := content[idx+2:]
	return path, body, nil
}

// ===================== Server =====================

type XServer struct {
	Addr   string
	Router *XRouter
}

func NewXServer(addr string, r *XRouter) *XServer {
	return &XServer{
		Addr:   addr,
		Router: r,
	}
}

func (s *XServer) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen failed addr=%s err=%w", s.Addr, err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			hlog.Error("[XHTTP] listener close error:", err)
		}
	}()

	fmt.Println("[XHTTP] Server listen on:", s.Addr)
	var acceptLimit = make(chan struct{}, 5000) // 最大5000连接
	for {
		conn, err := listener.Accept()
		if err != nil {
			if isTemporaryAcceptErr(err) {
				hlog.Error("[XHTTP] accept retryable error:", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			hlog.Error("[XHTTP] accept fatal error:", err)
			return err
		}
		go func(c net.Conn) {
			defer func() {
				if r := recover(); r != nil {
					hlog.Error("[XHTTP] panic recovered:", r)
				}
				hlog.Info("[XHTTP] connection closed:", c.RemoteAddr())
				<-acceptLimit
			}()

			s.handleConn(c)
		}(conn)
	}
}
func (s *XServer) handleConn(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	fmt.Printf("[XServer] 新连接: %s\n", remoteAddr)

	// 长连接！！！去掉读超时，不要主动断开！！！
	// ioTimeout := time.Duration(common.Config.RecvTimeout) * time.Second

	defer func() {
		_ = conn.Close()
		fmt.Printf("[XServer] 连接断开: %s\n", remoteAddr)
	}()

	// ==================== 正确做法：不超时，永久阻塞读 ====================
	for {
		// 读 4 字节 header（永久阻塞，不超时）
		header := make([]byte, 4)
		_, err := io.ReadFull(conn, header)
		if err != nil {
			// 只有客户端断开/错误才退出，不要主动超时断开！
			hlog.Errorf("[XServer] read header err: %v, addr=%s", err, remoteAddr)
			return
		}

		size := binary.BigEndian.Uint32(header)
		if size > uint32(maxPkgSize) || size == 0 {
			hlog.Error("[XServer] invalid pkg size:", size)
			return
		}

		// 读 body
		body := make([]byte, size)
		_, err = io.ReadFull(conn, body)
		if err != nil {
			hlog.Errorf("[XServer] read body err: %v, addr=%s", err, remoteAddr)
			return
		}

		// 解包
		path, data, err := DecodePkg(append(header, body...))
		if err != nil {
			hlog.Errorf("[XServer] decode err: %v", err)
			continue
		}

		// 同步/异步都行
		go s.Router.Dispatch(path, data)
	}
}

// ===================== Error =====================

func isTemporaryAcceptErr(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, syscall.EINTR) ||
		errors.Is(err, syscall.EMFILE) ||
		errors.Is(err, syscall.ENFILE) ||
		errors.Is(err, syscall.EAGAIN) {
		return true
	}

	return false
}
