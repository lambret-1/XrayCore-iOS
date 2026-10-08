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
	"runtime/debug"
	"strconv"
	"strings"
	"unsafe"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/ios"
)

var xrayInstance *core.Instance

func init() {
	// 限制 Go 堆内存软上限，防止运行时堆膨胀导致 RSS 持续走高
	// 当前观察：启动 31MB，稳定 44MB。设 40MB 上限后 GC 会更积极地
	// 回收并归还内存给系统，目标稳定在 36-40MB
	debug.SetMemoryLimit(40 << 20) // 40MB

	// GOGC 配合内存上限：100 表示堆达到活跃对象的 2 倍时触发 GC
	debug.SetGCPercent(100)
}

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

	// 关键：必须在 instance.Start() 之前设置 TUN fd 环境变量
	// Xray-core Darwin TUN 入站在 NewTun() 中读取 "xray.tun.fd" 获取外部 fd
	// 若在 Start 之后设置，TUN 入站已尝试创建新接口导致启动失败（错误码-4）
	os.Setenv("xray.tun.fd", strconv.Itoa(int(tunFd)))

	// 启动 Xray
	if err := instance.Start(); err != nil {
		return -4
	}

	xrayInstance = instance

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
