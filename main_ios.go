// 裁剪版 Xray 核心 iOS 静态库入口
// 仅保留 VLESS/VMess 协议，最小化内存占用
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"log"
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
	// iOS Packet Tunnel 扩展硬性内存上限约 50MB（jetsam 阈值）
	// Go 运行时软限制设为 28MB，留出约 22MB 给非 Go 内存
	// （系统框架、代码段、gVisor 栈、CGO 等）
	// 测速峰值 63MB 已超上限，必须压低 Go 堆目标
	debug.SetMemoryLimit(28 << 20) // 28MB

	// GOGC=50：堆达到活跃对象的 1.5 倍时触发 GC（默认 100 为 2 倍）
	// 更积极的 GC，降低堆峰值，CPU 开销在 VPN 场景可接受
	debug.SetGCPercent(50)
}

// 定期强制归还未使用内存给 OS，并输出详细 MemStats 诊断日志
// Go 默认 scavenger 保守，测速后大量 buffer 被 GC 回收但堆内存不归还 OS，
// 导致 Sys 值长期保持高位。定期 FreeOSMemory 强制 GC + madvise 归还。
func startMemoryScavenger() {
	memoryScavengerStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var before runtime.MemStats
				runtime.ReadMemStats(&before)

				debug.FreeOSMemory()

				var after runtime.MemStats
				runtime.ReadMemStats(&after)

				log.Printf("[MemScav] 回收前: HeapAlloc=%.1fMB HeapInuse=%.1fMB HeapIdle=%.1fMB HeapReleased=%.1fMB Sys=%.1fMB NumGC=%d Goroutines=%d",
					float64(before.HeapAlloc)/1048576,
					float64(before.HeapInuse)/1048576,
					float64(before.HeapIdle)/1048576,
					float64(before.HeapReleased)/1048576,
					float64(before.Sys)/1048576,
					before.NumGC,
					runtime.NumGoroutine())
				log.Printf("[MemScav] 回收后: HeapAlloc=%.1fMB HeapInuse=%.1fMB HeapIdle=%.1fMB HeapReleased=%.1fMB Sys=%.1fMB 归还=%.1fMB",
					float64(after.HeapAlloc)/1048576,
					float64(after.HeapInuse)/1048576,
					float64(after.HeapIdle)/1048576,
					float64(after.HeapReleased)/1048576,
					float64(after.Sys)/1048576,
					float64(before.HeapReleased-after.HeapReleased)/1048576)
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

	// 启动定期内存归还（每30秒强制 FreeOSMemory + 详细诊断日志）
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
