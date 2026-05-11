package common

/*
 * Copyright (c) 2026 Corey <corey101@qq.com>
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
import (
	"flag"
	"fmt"
	"github.com/koding/multiconfig"
	"os"
)

var Config *ServerConfig

type FlagConfig struct {
	ConfigFile string `default:"config.json"`
}

type ServerConfig struct {
	ForwardAddr string //转发地址
	RecvPort    string `default:":9100"` //接收端口
	SendEnabled bool   //发
	RecvEnabled bool   //收
	IdleTimeout int    //客户端空闲连接超时
	PrintStat   bool   //打印状态
	Debug       bool
	LogLevel    int
	AccessLog   string
}

func CheckErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}
func LoadConfig() error {
	Config = &ServerConfig{}
	return Config.load()
}

func (c *FlagConfig) load() error {
	t := &multiconfig.TagLoader{}
	f := &multiconfig.FlagLoader{}
	m := multiconfig.MultiLoader(t, f)
	if err := m.Load(c); err == flag.ErrHelp {
		os.Exit(0)
	} else if err != nil {
		return err
	}
	return nil
}

func (c *ServerConfig) load() error {
	//加载配置文件路径
	f := &FlagConfig{}
	err := f.load()
	if err == flag.ErrHelp {
		os.Exit(0)
	} else if err != nil {
		return err
	}
	t := &multiconfig.TagLoader{}
	j := &multiconfig.JSONLoader{Path: f.ConfigFile}
	m := multiconfig.MultiLoader(t, j)
	//加载到结构变量内
	err = m.Load(c)
	if err != nil {
		return err
	}
	return err
}
