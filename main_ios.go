// 裁剪版 Xray 核心 iOS 静态库入口
// 仅保留 VLESS/VMess 协议，最小化内存占用
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"os"
	"runtime"
	"strings"
	"unsafe"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/all"
)

var xrayInstance *core.Instance

//export StartXray
func StartXray(configStr *C.char, tunFd C.int) C.int {
	if xrayInstance != nil {
		return 0
	}

	configJSON := C.GoString(configStr)

	// 解析配置
	config, err := serial.DecodeJSONConfig(strings.NewReader(configJSON))
	if err != nil {
		return -1
	}

	pbConfig, err := config.Build()
	if err != nil {
		return -2
	}

	// 创建 Xray 实例
	instance, err := core.New(pbConfig)
	if err != nil {
		return -3
	}

	// 启动 Xray
	if err := instance.Start(); err != nil {
		return -4
	}

	xrayInstance = instance

	// 设置 TUN 文件描述符（通过环境变量传递给 tun 入站）
	os.Setenv("XRAY_TUN_FD", string(rune(tunFd)))

	return 0
}

//export StopXray
func StopXray() C.int {
	if xrayInstance == nil {
		return 0
	}

	if err := xrayInstance.Close(); err != nil {
		return -1
	}

	xrayInstance = nil
	return 0
}

//export GetVersion
func GetVersion() *C.char {
	version := core.Version()
	return C.CString(version)
}

//export FreeString
func FreeString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

//export QueryStats
func QueryStats(outboundTag *C.char) *C.char {
	// 流量统计在 TUN 模式下不可用，返回空 JSON
	return C.CString(`{"uplink":0,"downlink":0}`)
}

//export CurrentRSSBytes
func CurrentRSSBytes() C.ulonglong {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return C.ulonglong(stats.Sys)
}

func main() {}
