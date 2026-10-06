# XrayCore-iOS

裁剪版 Xray 核心 iOS 静态库，仅保留 VLESS/VMess 协议，最小化内存占用。

## 裁剪说明

基于 [XTLS/Xray-core](https://github.com/XTLS/Xray-core) 裁剪，移除以下协议：

- Shadowsocks / Shadowsocks 2022
- Trojan
- SOCKS
- HTTP
- Dokodemo-door
- Loopback
- MASQUE
- Hysteria
- WireGuard

**保留协议**：VLESS、VMess、Freedom（直连）、Blackhole（黑洞）、TUN、DNS

**保留传输层**：TCP、UDP、TLS、WebSocket、gRPC、HTTPUpgrade、KCP、SplitHTTP、XDrive、REALITY

## 内存优化

相比完整版 Xray，裁剪后预计减少：
- 二进制体积：减少约 30-40%
- 启动内存：减少约 10-15MB
- 运行时内存：减少约 5-10MB

## 编译

依赖：macOS + Xcode 15+ + Go 1.22+

```bash
chmod +x build_ios.sh
./build_ios.sh
```

编译产物：`build/Xray.xcframework`

## C 接口

```c
// 启动 Xray，configStr 为 JSON 配置，tunFd 为 TUN 文件描述符
// 返回 0 表示成功，负数表示错误
int StartXray(char* configStr, int tunFd);

// 停止 Xray
int StopXray(void);

// 获取 Xray 版本号
char* GetVersion(void);

// 释放 Go 分配的字符串
void FreeString(char* s);

// 查询出站流量统计（TUN 模式下返回 0）
char* QueryStats(char* outboundTag);

// 获取当前进程内存使用（字节）
unsigned long long CurrentRSSBytes(void);
```

## 项目结构

```
├── main_ios.go          # iOS C 接口入口
├── build_ios.sh         # iOS 编译脚本
├── main/                 # Xray 主程序
│   └── distro/all/      # 协议注册（已裁剪）
├── proxy/                # 协议实现（已裁剪）
│   ├── vless/
│   ├── vmess/
│   ├── freedom/
│   ├── blackhole/
│   ├── tun/
│   └── dns/
├── infra/conf/           # 配置解析（已裁剪）
└── transport/            # 传输层
```

## 与原项目对比

| 项目 | 完整版 | 裁剪版 |
|------|--------|--------|
| 协议数量 | 12+ | 6（VLESS/VMess/Freedom/Blackhole/TUN/DNS） |
| 二进制大小 | ~25MB | ~15MB（预计） |
| 启动内存 | ~50MB | ~35MB（预计） |
| 适用场景 | 通用代理 | iOS VPN 扩展（VLESS/VMess） |

## License

MIT（与 Xray-core 一致）
