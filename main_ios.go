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
	"time"
	"unsafe"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/ios"
)

var xrayInstance *core.Instance
var memoryScavengerStop chan struct{}

func init() {
	// 限制 Go 堆内存软上限，防止运行时堆膨胀导致 RSS 持续走高
	debug.SetMemoryLimit(40 << 20) // 40MB

	// GOGC 配合内存上限：100 表示堆达到活跃对象的 2 倍时触发 GC
	debug.SetGCPercent(100)
}

// 定期强制归还未使用内存给 OS
// Go 默认 scavenger 比较保守，测速后大量 buffer 被 GC 回收但堆内存不归还 OS，
// 导致 Sys 值长期保持高位。定期 FreeOSMemory 强制 GC + madvise 归还。
func startMemoryScavenger() {
	memoryScavengerStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				debug.FreeOSMemory()
			case <-memoryScavengerStop:
				return
			}
		}
	}()
}

func stopMemoryScavenger() {
	if memoryScavengerStop != nil {
		close(memoryScavengerStop)
		memoryScavengerStop = nil
	}
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

	// 启动定期内存归还（每30秒强制 FreeOSMemory）
	startMemoryScavenger()

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

	// 停止内存回收 goroutine 并强制归还一次
	stopMemoryScavenger()
	debug.FreeOSMemory()

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
