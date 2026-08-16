# CubeSandbox 部署 SOP (Loom v2.0 沙箱层)

> 决策日期: 2026-08-01
> 决策: **v2.0 spec §3 单一后端 = CubeSandbox** (v2.0 阶段不再兼容 AgentENV / E2B, YAGNI)
> 仓库: https://github.com/TencentCloud/CubeSandbox (本地 clone: `~/Documents/Codex/2026-06-18/cubesandbox/`)
> License: **Apache 2.0** (LICENSE 文件确认)
> 最近 commit: 2026-08-01 (活跃, 10.8k stars, CNCF Landscape)

## 0. 部署前置 (一锤定音)

| 需求 | 现实 |
|---|---|
| macOS 主机直接跑 | ❌ 不支持 (无 /dev/kvm) |
| macOS 内的 Linux VM (UTM/Parallels) | ❌ 不支持 (嵌套 virt 不通) |
| Linux 物理机 | ✅ |
| Linux x86_64 VM (EC2/GCE/Azure/Hetzner) | ✅ 标准 KVM |
| **Linux x86_64 云 CVM (50元/月, 无 /dev/kvm)** | ✅ **PVM 模式 (CubeSandbox 独有)** |
| Linux arm64 VM (Graviton) | ✅ v0.5+ 支持 |

**macOS 用户的唯一现实路径**: 你 (Apple Silicon) → 沙箱在远程 Linux VM 跑 → Loom daemon 跨 SSH 调 HTTP API

## 1. 三种部署路径对比

| 路径 | 适合 | 成本 | 启动时间 | 维护难度 |
|---|---|---|---|---|
| **A. macOS 本地 OrbStack Linux VM** (用 dev-env QEMU) | 调试 / 二次开发 | $0 | 10 min | 中 |
| **B. 腾讯云 CVM (PVM 模式)** | 生产 / 真实跑 | 50-100元/月 | 3 min | 低 |
| **C. 用户已有 Linux 服务器** | 自托管 | $0 | 5 min | 低 |

**Loom v2.0 spec 推荐**: 阶段 2.A-2.C 用 **A 调试**; 阶段 2.D 验证用 **B 生产**。

## 2. 路径 A — macOS 本地 OrbStack + dev-env QEMU 调试 (10 分钟)

### 前置
```bash
# 1. macOS 上安装 OrbStack (推荐, 比 Colima 更快, 集成 KVM)
brew install orb   # 或从 https://orbstack.dev 下载

# 2. 启动 OrbStack Linux VM
orbctl launch orb  # 默认 Debian/Ubuntu

# 3. 进 OrbStack 内部
orbctl shell orb
```

### 5 步部署 (在 OrbStack Linux 内部)
```bash
# Step 1: clone CubeSandbox (10.8k stars, 完整仓库 ~50MB)
cd ~
git clone --depth=1 https://github.com/TencentCloud/CubeSandbox.git
cd CubeSandbox

# Step 2: 用 dev-env 准备 OpenCloudOS VM 镜像 (一次性, ~10 分钟)
cd dev-env
./prepare_image.sh         # 下载 + 配置 OpenCloudOS 9 + kvm nested

# Step 3: 启动 OpenCloudOS VM (一个终端)
./run_vm.sh                # 端口转发: SSH 10022, Cube API 13000, WebUI 12088

# Step 4: 另一个终端登录
./login.sh                 # 自动 root 登录

# Step 5: VM 内装 CubeSandbox (一次性, 3 分钟)
curl -sL https://github.com/tencentcloud/CubeSandbox/raw/master/deploy/one-click/online-install.sh | bash

# 验证
curl -sf http://127.0.0.1:3000/health && echo OK
```

### Loom 集成验证 (macOS 端, 装 E2B SDK)
```bash
# macOS 装 E2B Python SDK
pip install e2b-code-interpreter

# 连接到 OrbStack 里的 CubeSandbox VM
export E2B_API_URL=http://127.0.0.1:13000      # dev-env 端口转发
export E2B_API_KEY=e2b_0000000000000000000000000000000000000000  # dev mode 任意

# 模板创建 (VM 内)
ssh -p 10022 root@127.0.0.1 "cubemastercli tpl create-from-image \
  --image ccr.ccs.tencentyun.com/ags-image/sandbox-code:latest \
  --writable-layer-size 1G --expose-port 49999 --expose-port 49983 --probe 49999"

# 跑通 hello
python -c "
from e2b_code_interpreter import Sandbox
import os
os.environ['CUBE_TEMPLATE_ID'] = '<上一步返回的 template_id>'
with Sandbox.create(template=os.environ['CUBE_TEMPLATE_ID']) as sb:
    r = sb.run_code('print(\"Hello from CubeSandbox via E2B SDK!\")')
    print(r.logs.stdout)
"
```

## 3. 路径 B — 腾讯云 CVM (PVM 模式, 3 分钟, 推荐生产)

### 前置
- 腾讯云账号 (lune 已有)
- 买 1 台 CVM: 镜像 **OpenCloudOS 9.4**, 配置 2C4G 起步, 公网带宽 5Mbps
- 成本: 50-100 元/月 (轻量应用服务器)

### 5 条命令部署 (在 CVM 上, 一行一回车)
```bash
# 1. 装 PVM 宿主机内核 (OpenCloudOS 9 直接进 yum 仓库)
sudo dnf install -y kernel-6.6.69-1.1.cubesandbox.oc9 \
  kernel-core-6.6.69-1.1.cubesandbox.oc9 \
  kernel-modules-6.6.69-1.1.cubesandbox.oc9

# 2. 配置 grub + reboot
curl -sL https://cnb.cool/CubeSandbox/CubeSandbox/-/git/raw/master/deploy/pvm/grub/host_grub_config.sh | sudo bash
sudo reboot
# 等 30 秒, SSH 重新连

# 3. 加载 kvm_pvm 模块
sudo modprobe kvm_pvm
echo 'kvm_pvm' | sudo tee /etc/modules-load.d/kvm-pvm.conf
ls -l /dev/kvm   # 应存在

# 4. 一键安装 CubeSandbox (PVM 模式)
curl -sL https://cnb.cool/CubeSandbox/CubeSandbox/-/git/raw/master/deploy/one-click/online-install.sh \
  | CUBE_PVM_ENABLE=1 MIRROR=cn sudo bash
# 等 3 分钟

# 5. 验证
curl -sf http://127.0.0.1:3000/health && echo OK
# 看到 4 个进程: network-agent / cubemaster / cube-api / cubelet
ps aux | grep -E "cubemaster|cube-api|cubelet|network-agent" | grep -v grep
```

### Loom daemon 集成 (macOS 端)
```python
# Loom daemon 通过 SSH 跳板 + HTTP API 调 CubeSandbox
import os
os.environ["E2B_API_URL"] = "http://<CVM 公网 IP>:3000"   # 或内网 IP
os.environ["E2B_API_KEY"] = "e2b_<daemon 启动时生成的 token>"
os.environ["CUBE_TEMPLATE_ID"] = "<cubemastercli 创建的 template_id>"

# 直接用 E2B SDK 调
from e2b_code_interpreter import Sandbox
with Sandbox.create(template=os.environ["CUBE_TEMPLATE_ID"]) as sb:
    result = sb.run_code("print('Loom daemon → CubeSandbox → OK')")
    print(result.logs.stdout)
```

## 4. 路径 C — 用户已有 Linux 服务器 (5 分钟)

如果用户已有 Linux 主机 / 服务器:
```bash
# 1. 装 Docker + curl + sudo 权限
# 2. 直接装 (普通 KVM 模式, 不需要 PVM)
curl -sL https://github.com/TencentCloud/CubeSandbox/raw/master/deploy/one-click/online-install.sh | sudo bash
# 3. 验证
curl -sf http://127.0.0.1:3000/health && echo OK
```

## 5. 集成到 Loom v2.0 spec §3 — 单后端最小设计

```go
// internal/sandbox/cubesandbox.go (v2.0 spec §3 唯一实现)
package sandbox

import (
    "context"
    "os"
    "github.com/e2b-dev/go-sdk"
)

type CubeSandboxClient struct {
    client *e2b.Sandbox
}

func (c *CubeSandboxClient) Start(template string, opts ...StartOpt) (SandboxID, error) {
    sb, err := e2b.Sandbox.Create(context.Background(), template, &e2b.SandboxCreateOpts{
        // Loom 特定配置: 超时 / 凭证 / 挂载
    })
    return SandboxID(sb.ID), err
}

// 其他方法: Pause / Resume / Exec / Delete / Timeout 都从 E2B SDK 包
```

**v2.0 spec §3 单一后端** 写入 (替换原 §3.2-3.4 双/三 backend):
```markdown
§3 Sandbox 层 — 单一 CubeSandbox 后端
3.1 抽象层 (E2B Compatible Go interface)
3.2 Backend: TencentCloud/CubeSandbox
3.3 部署路径 (3 选 1, 见 .loom-drafts/cubesandbox-deploy-sop.md)
3.4 凭证流 (借鉴 CubeEgress 凭证保险库, 详见 spec §2.4)
3.5 选型理由 (10.8k stars + CNCF + PVM 部署 + 凭证安全 + Apache 2.0)
```

## 6. 验证清单 (v2.0 spec §3 实施完必跑)

- [ ] `curl http://<host>:3000/health` 返回 OK
- [ ] `ps aux | grep cube-` 显示 4 个核心进程
- [ ] `cubemastercli tpl list` 能看到至少 1 个 READY 模板
- [ ] `python -c "from e2b_code_interpreter import Sandbox; sb = Sandbox.create(template=os.environ['CUBE_TEMPLATE_ID']); print(sb.run_code('1+1').text)"` 输出 2
- [ ] 网络隔离: `sb.run_code("curl 169.254.169.254")` 应被 CubeVS 拦截
- [ ] 凭证隔离: `sb.files.read_file('/run/secrets/api_key')` 不存在 (凭证由 CubeEgress 注入)
- [ ] 冷启动: 10 次平均 < 100ms (PVM 模式应 < 80ms)
- [ ] 内存开销: `ps -o rss` 每个沙箱 < 10MB

## 7. 下一步

- 跑通后, 写一个 `internal/sandbox/cubesandbox.go` 最小 wrapper
- 在 Loom v2.0 spec §3 替换双 backend 设计为单 CubeSandbox
- 把"3 backend 兼容"的 spec 文字改成"单一 CubeSandbox, 留 escape hatch 到 AgentENV 作为 v2.1+ 候选"

## 8. 已知风险与回滚

| 风险 | 概率 | 缓解 | 回滚 |
|---|---|---|---|
| 腾讯云 CVM PVM 内核突然无维护 | 低 | 锁 kernel 版本, 升级先在 stage 测 | 切回普通 KVM (换 CVM 类型) |
| CubeSandbox Apache 2.0 NOTICE 漏写 | 中 | 走 license 检查, 自动加 NOTICE | Loom 不分发现成的 |
| E2B API 协议升级 | 中 | 锁 SDK 版本, 升级有 breaking change 提示 | 加 adapter 层隔离 |
| 腾讯云账户欠费 / CVM 释放 | 低 | 启用自动续费, 监控告警 | 数据走快照, 重建 CVM 5 分钟 |

## VERDICT: READY_FOR_REVIEW
