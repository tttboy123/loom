# Phase 2 Goal: 5 新云 × 4 工具 MCP Server 全实现

> **触发**: 用户说"立即跑Phase 2"
> **当前状态**: Phase 1 (tencent-cloud) ✅ + GitHub 仓库 ✅
> **目标**: 5 个新云 (alicloud/google/aws/azure/baiducloud) × 4 工具 = 20 工具

## 当前状态（不要重做）

✅ **已存在**:
- `~/code/cloud-skills-mcp/` git 仓库
- `internal/mcp/sdk/` 共享 SDK (creds/tools/errors/apidoc)
- `internal/mcp/tencent/main.go` 4 工具 PoC (list/describe/start/stop) — **作为模板**
- `~/bin/tencent-cloud-mcp` 8.5MB binary
- GitHub: https://github.com/tttboy123/cloud-skills-mcp (2 commits)
- 2 个 atomic commits: `4cbc94a` (scaffold) + `4ae270b` (Phase 1)

✅ **关键修复**（遵守）:
- tccli env var 必须 `TENCENTCLOUD_SECRET_ID` (带下划线)
- macOS BSD date 用 `-r @timestamp`
- macOS LibreSSL 用 `awk '{print $NF}'`
- bash pipe 不用 `if cmd | head`，用 mktemp + 同步调用 + 真 exit code
- token 不持久化, push 完立即改回 SSH remote

## 本次目标 (Phase 2 完整)

### 5 个新云 × 4 工具 = 20 工具

每个云都是同样的 4 工具模板：

| # | 工具命名规则 | 例 (alicloud) | 例 (aws) |
|---|---|---|---|
| 1 | `<cloud>_<service>_list` | `alicloud_ecs_list_instances` | `aws_ec2_list_instances` |
| 2 | `<cloud>_<service>_describe` | `alicloud_ecs_describe_instance` | `aws_ec2_describe_instance` |
| 3 | `<cloud>_<service>_start` (mutating, --force) | `alicloud_ecs_start_instance` | `aws_ec2_start_instance` |
| 4 | `<cloud>_<service>_stop` (mutating, --force) | `alicloud_ecs_stop_instance` | `aws_ec2_stop_instance` |

### 5 个新云的具体命令

| 云 | CLI | Compute 工具调用 | 凭证 env var |
|---|---|---|---|
| **alicloud** | `aliyun` | `aliyun ecs DescribeInstances --RegionId cn-hangzhou` | `ALIBABACLOUD_ACCESS_KEY_ID` (带下划线!) |
| **google** | `gcloud` | `gcloud compute instances list --project=... --zone=...` | `GOOGLE_APPLICATION_CREDENTIALS` |
| **aws** | `aws` | `aws ec2 describe-instances --region us-east-1` | `AWS_ACCESS_KEY_ID` |
| **azure** | `az` | `az vm list` | `AZURE_SUBSCRIPTION_ID` |
| **baiducloud** | `bcecmd` | `bcecmd bcc ListInstance` | `BCE_ACCESS_KEY_ID` |

**重要**: 如果 CLI 不在 PATH, MCP server 返回友好错误 `"CLI not found: install via <url>"`, 不要 panic。

## 文件结构（按 tencent 模板复制）

```
~/code/cloud-skills-mcp/
├── internal/mcp/
│   ├── sdk/                        # 已有, 不要改
│   ├── tencent/main.go             # 已有, 不要改
│   ├── alicloud/main.go            # 新建 (~250 行)
│   ├── google/main.go              # 新建 (~250 行)
│   ├── aws/main.go                 # 新建 (~250 行)
│   ├── azure/main.go               # 新建 (~250 行)
│   └── baidu/main.go               # 新建 (~250 行)
├── tencent-cloud/SKILL.md          # 已有
├── alicloud/SKILL.md               # 已有 meta, 加 MCP 章节
├── google-cloud/SKILL.md           # 已有 meta, 加 MCP 章节
├── aws/SKILL.md                    # 新建
├── azure/SKILL.md                  # 新建
├── baiducloud/SKILL.md             # 新建
├── go.mod                          # 已有
├── go.sum                          # 已有
├── docs/
│   ├── cloud-skills-mcp-design.md  # 已有
│   ├── cloud-mcp-poc-deliverable.md # 已有
│   └── cloud-mcp-phase2-deliverable.md  # 新建
└── .gitignore                      # 加 */mcp 二进制
```

## 实施步骤

### Step 1: 复制 tencent/main.go 模板到 5 个新云

**alicloud/main.go** (~250 行):
- 改 `service="cvm"` → `service="ecs"` + `host="ecs.<region>.aliyuncs.com"`
- 改 `tccliPath = "tccli"` → `tccliPath = "aliyun"`
- 改 `aliyun ecs DescribeInstances --RegionId <region> --InstanceIds.0 <id>` 等
- 改 `serverName = "alicloud-mcp"`
- 改 `LoadCreds("alicloud")`
- 凭证 export 改 `ALIBABACLOUD_ACCESS_KEY_ID` / `ALIBABACLOUD_ACCESS_KEY_SECRET`
- 工具命名: `alicloud_ecs_list_instances` / `alicloud_ecs_describe_instance` / `alicloud_ecs_start_instance` / `alicloud_ecs_stop_instance`

**google/main.go**:
- 改 `gcloudPath = "gcloud"`
- 改 `gcloud compute instances list --project=<p> --zone=<z>`
- 改 `gcloud compute instances describe <name> --project=<p> --zone=<z>`
- 改 `gcloud compute instances start <name> --project=<p> --zone=<z>` (--force)
- 改 `gcloud compute instances stop <name> --project=<p> --zone=<z>` (--force)
- 凭证: `GOOGLE_APPLICATION_CREDENTIALS` (file path)
- 工具命名: `google_compute_list_instances` / `google_compute_describe_instance` / `google_compute_start_instance` / `google_compute_stop_instance`

**aws/main.go**:
- 改 `awsPath = "aws"`
- 改 `aws ec2 describe-instances --region <region> --instance-ids <id>` (注: --instance-ids 是复数, 但这里只 1 个)
- 改 `aws ec2 start-instances --region <region> --instance-ids <id>` (--force)
- 改 `aws ec2 stop-instances --region <region> --instance-ids <id>` (--force)
- 凭证: `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_REGION`
- 工具命名: `aws_ec2_list_instances` / `aws_ec2_describe_instance` / `aws_ec2_start_instance` / `aws_ec2_stop_instance`

**azure/main.go**:
- 改 `azPath = "az"`
- 改 `az vm list`
- 改 `az vm show --name <name> --resource-group <rg>`
- 改 `az vm start --name <name> --resource-group <rg>` (--force)
- 改 `az vm stop --name <name> --resource-group <rg>` (--force)
- 凭证: `AZURE_SUBSCRIPTION_ID` / `AZURE_TENANT_ID` / `AZURE_CLIENT_ID` / `AZURE_CLIENT_SECRET` (或用 `az login` 缓存)
- 工具命名: `azure_vm_list` / `azure_vm_show` / `azure_vm_start` / `azure_vm_stop`

**baidu/main.go**:
- 改 `bcecmdPath = "bcecmd"`
- 改 `bcecmd bcc ListInstance`
- 改 `bcecmd bcc GetInstanceDetail --instanceId <id>`
- 改 `bcecmd bcc StartInstance --instanceId <id>` (--force)
- 改 `bcecmd bcc StopInstance --instanceId <id>` (--force)
- 凭证: `BCE_ACCESS_KEY_ID` / `BCE_SECRET_ACCESS_KEY` / `BCE_REGION` (默认 cn-bj)
- 工具命名: `baiducloud_bcc_list` / `baiducloud_bcc_describe` / `baiducloud_bcc_start` / `baiducloud_bcc_stop`

### Step 2: 编译 5 个二进制

```bash
cd ~/code/cloud-skills-mcp

for cloud in alicloud google aws azure baidu; do
  echo "Building ${cloud}-mcp..."
  go build -o ~/bin/${cloud}-mcp ./internal/mcp/${cloud}/
done

ls -la ~/bin/*-mcp
# 期望: 6 个 binary 都在
```

### Step 3: 端到端测 5 个新云 (假设凭证缺失, 应返回 friendly error)

```bash
# 5 个 daemon 都能 --help exit 0
for cloud in alicloud google aws azure baidu; do
  echo "--- ${cloud} ---"
  ~/bin/${cloud}-mcp --help 2>&1 | head -5
done

# 5 个 daemon 都能 tools/list 返回 4 工具
for cloud in alicloud google aws azure baidu; do
  echo "--- ${cloud} tools/list ---"
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ~/bin/${cloud}-mcp | head -5
done

# 5 个 daemon 都能 tools/call 调 list 工具 (凭证缺失返回 friendly error)
for cloud in alicloud google aws azure baidu; do
  echo "--- ${cloud} list call ---"
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"'${cloud}'_list","arguments":{}}}' | ~/bin/${cloud}-mcp 2>&1 | head -5
done
```

### Step 4: 写 5 个 SKILL.md (参考 tencent-cloud/SKILL.md 模板)

每个 SKILL.md:
- YAML frontmatter (name, description, allowed-tools, license)
- 快速开始 (clone + 配置 + 调 MCP)
- 4 工具列表
- 凭证配置
- 跟 tencent / alicloud / google 对比
- Loom 集成占位

### Step 5: 写 deliverable

`docs/cloud-mcp-phase2-deliverable.md` (类似 Phase 1 格式):
- 完成的工作 (5 个 daemon + 5 个 SKILL.md + build script)
- 编译输出 (5 个 binary)
- 端到端测试输出 (5 云的 --help / tools/list / tools/call)
- VERDICT: PASS

### Step 6: Git commit + push (注意 no-auto-push 规则)

**CRITICAL: 不要 push 到 remote! 用户授权 push 时再 push**

```bash
cd ~/code/cloud-skills-mcp
git add -A
git status --short

# 1 个 commit 包含所有 5 云的实现
git commit -m "feat(mcp): Phase 2 - 5 new cloud MCP servers (20 tools total)

- alicloud/mcp    4 tools (ecs_list/describe/start/stop)
- google/mcp      4 tools (compute_list/describe/start/stop)
- aws/mcp         4 tools (ec2_list/describe/start/stop)
- azure/mcp       4 tools (vm_list/show/start/stop)
- baidu/mcp       4 tools (bcc_list/describe/start/stop)

Total: 6 daemons, 24 tools (incl. tencent)
Binaries: ~/bin/{alicloud,google,aws,azure,baidu,tencent}-mcp

Each daemon:
- Reuses shared SDK (creds + tools + errors)
- 3-tier credential loading (Keychain + env + native CLI config)
- Requires --force=true for mutating operations
- AKSK redaction in error messages

Verification: see docs/cloud-mcp-phase2-deliverable.md
VERDICT: PASS

Co-Authored-By: Claude (Mavis Code) <noreply@anthropic.com>"

# ⚠️ 不要 push! 留本地, 等用户授权
```

## 硬规则

1. **VERDICT 必须有** — 末尾 `VERDICT: PASS` 或 `FAIL`
2. **实际跑出来的输出** — 不是 "should work" / "compiled"
3. **凭证零暴露** — 用 `<AKID>` 占位
4. **mavis-trash 不用 rm**
5. **trap 不用 single quote**
6. **bash pipe 不用 if**
7. **早期 + 频繁 atomic commits** — 每完成 1 个 daemon 就 commit 1 次 (5 个 commits 总)
8. **deliverable.md 在 commit 前先写好**
9. **不要 push 到 remote** — 留给用户

## 跨项目 lesson 速查

- macOS BSD date: `-r @timestamp`, 不用 `-d`
- macOS LibreSSL: `awk '{print $NF}'`, 不用 `'{print $2}'`
- tccli/aliyun/aws env var: 各家都有特殊名字 (TENCENTCLOUD_SECRET_ID, ALIBABACLOUD_ACCESS_KEY_ID, AWS_ACCESS_KEY_ID 等)
- bash echo 默认不解释 \n, 用 printf
- mavis-trash wrapper 把 single quote 里的 ${VAR} 当字面量
- 2>&1 | jq 模式吞错, 用 mktemp + 真 exit code

## 完成判据

1. ✅ 5 个新 daemon 都编译 exit 0
2. ✅ 5 个新 daemon 都能 --help 返回工具列表
3. ✅ 5 个新 daemon 都能 tools/list 返回 4 工具
4. ✅ 5 个新 daemon 都能 tools/call list 工具 (凭证缺失返回 friendly error, 不 panic)
5. ✅ 5 个新 SKILL.md 都写好
6. ✅ deliverable.md 存在, 末尾 `VERDICT: PASS`
7. ✅ 6 提交 (1 commit per daemon) + 1 文档 commit = 6 atomic commits
8. ✅ 没 push 到 remote
9. ✅ 凭证零暴露

## 报告方式

完成后报告:
- 5 个 daemon 的 go build 输出
- 5 个 daemon 的 --help 输出
- 5 个 daemon 的 tools/list JSON
- 5 个 daemon 的 tools/call list 工具输出
- 5 个 SKILL.md 路径
- deliverable.md 路径
- 5 个 atomic commit hashes
- 任何踩到的坑 + 修法
