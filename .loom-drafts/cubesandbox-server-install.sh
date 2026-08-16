#!/usr/bin/env bash
# CubeSandbox 一键安装脚本 (路径 C — 已有 Linux 服务器)
# 用途: 在 Linux 服务器 (Ubuntu/CentOS/OpenCloudOS) 上一键装好 CubeSandbox
# 跑法: SSH 上服务器后, 复制粘贴下面这整段运行, 或:
#       curl -fsSL <url> | bash
# 期望: 5-10 分钟跑完; 跑完打印 E2B endpoint 给 Loom daemon 用

set -euo pipefail

echo "============================================"
echo "CubeSandbox 一键安装 — Loom v2.0 沙箱层"
echo "============================================"
echo ""

# 0. 环境检查
echo "🔍 Step 0: 环境检查"
if [[ $EUID -ne 0 ]]; then
  echo "❌ 请以 root 跑: sudo bash $0"
  exit 1
fi

if [[ ! -e /dev/kvm ]]; then
  echo "❌ /dev/kvm 不存在, 硬件虚拟化没开"
  echo "   请检查 BIOS 开启 VT-x/AMD-V + 内核加载 kvm 模块"
  exit 1
fi

if ! lsmod | grep -q kvm; then
  echo "⚠️  kvm 模块没加载, 尝试 modprobe"
  modprobe kvm 2>/dev/null || modprobe kvm_intel 2>/dev/null || modprobe kvm_amd 2>/dev/null || true
fi

echo "✅ /dev/kvm 存在"

# 1. Docker 检查
echo ""
echo "🔍 Step 1: Docker 检查"
if ! command -v docker &> /dev/null; then
  echo "⚠️  Docker 没装, 开始装"
  if command -v dnf &> /dev/null; then
    dnf install -y docker docker-compose
    systemctl enable --now docker
  elif command -v apt &> /dev/null; then
    apt update
    apt install -y docker.io docker-compose-v2
    systemctl enable --now docker
  else
    echo "❌ 不知道用 dnf 还是 apt, 请手动装 docker"
    exit 1
  fi
else
  echo "✅ Docker 已装: $(docker --version)"
fi

# 2. curl + git 检查
echo ""
echo "🔍 Step 2: 工具检查"
for cmd in curl sudo; do
  if ! command -v $cmd &> /dev/null; then
    echo "❌ $cmd 没装, 请 apt/dnf install $cmd"
    exit 1
  fi
done
echo "✅ curl + sudo 都齐"

# 3. 一键安装 CubeSandbox
echo ""
echo "🚀 Step 3: 一键安装 CubeSandbox (3-5 分钟)"
cd /tmp
curl -sL https://github.com/TencentCloud/CubeSandbox/raw/master/deploy/one-click/online-install.sh | bash
echo "✅ online-install.sh 跑完"

# 4. 验证服务
echo ""
echo "🔍 Step 4: 验证 4 个核心进程"
sleep 5  # 等等 systemd 起来
for proc in cubemaster cube-api cubelet network-agent; do
  if pgrep -f $proc > /dev/null; then
    echo "  ✅ $proc 在跑"
  else
    echo "  ❌ $proc 没起来, 看 /data/log/"
  fi
done

# 5. 健康检查
echo ""
echo "🔍 Step 5: 健康检查"
if curl -sf http://127.0.0.1:3000/health; then
  echo " ✅ OK"
else
  echo " ❌ /health 没通, 看 /data/log/ 日志"
  exit 1
fi

# 6. 打印 E2B endpoint 给 Loom daemon
echo ""
echo "============================================"
echo "🎉 装好啦! 给 Loom daemon 用的信息:"
echo "============================================"
SERVER_IP=$(hostname -I | awk '{print $1}')
echo ""
echo "E2B_API_URL=http://${SERVER_IP}:3000"
echo "E2B_SANDBOX_URL=http://${SERVER_IP}:3000"
echo "E2B_API_KEY=e2b_0000000000000000000000000000000000000000   # dev mode 任意非空即可"
echo "E2B_ACCESS_TOKEN=dummy   # dev mode 任意"
echo ""
echo "👉 把这 4 行环境变量设到 Loom daemon 启动脚本里"

# 7. 创建第一个代码执行模板
echo ""
echo "🔍 Step 7: 创建代码执行模板 (sandbox-code:latest)"
if command -v cubemastercli &> /dev/null; then
  TPL_OUT=$(cubemastercli tpl create-from-image \
    --image ccr.ccs.tencentyun.com/ags-image/sandbox-code:latest \
    --writable-layer-size 1G \
    --expose-port 49999 \
    --expose-port 49983 \
    --probe 49999 2>&1) || true
  echo "$TPL_OUT"
  TPL_ID=$(echo "$TPL_OUT" | grep -oP 'template_id: \K.*' | head -1 || echo "")
  if [[ -n "$TPL_ID" ]]; then
    echo ""
    echo "✅ 模板创建好, 等 1-2 分钟到 READY 状态"
    echo "👉 CUBE_TEMPLATE_ID=$TPL_ID (设到 Loom daemon)"
  fi
else
  echo "⚠️  cubemastercli 不在 PATH, 手动创建模板"
fi

# 8. WebUI 提示
echo ""
echo "============================================"
echo "📊 WebUI 访问:"
echo "   http://${SERVER_IP}:12088"
echo "   (默认用户 admin, 密码在 /data/cube-sandbox/.env 文件里)"
echo "============================================"

echo ""
echo "✅ 一键安装完成! 跑通验证:"
echo "   curl -sf http://127.0.0.1:3000/health && echo OK"
echo "✅ Loom 集成测试 (在 macOS 端):"
echo "   export E2B_API_URL=http://${SERVER_IP}:3000"
echo "   pip install e2b-code-interpreter"
echo "   python -c \"from e2b_code_interpreter import Sandbox; sb = Sandbox.create(template='<TPL_ID>'); print(sb.run_code('1+1').text)\""

exit 0
