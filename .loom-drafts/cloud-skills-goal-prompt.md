# Goal 提示词: 6 大云厂商 MCP + Skills 双形态

> **使用方式**: 复制下面的 `<GOAL>` 块，粘贴到新 Session 的第一条消息
> **关联文档**: `.loom-drafts/cloud-skills-mcp-design.md`

---

<GOAL>

## 任务

为 6 大云厂商（Google Cloud / Azure / AWS / AliCloud / TencentCloud / BaiduCloud BCE）搭建 **SKILL.md + MCP server 双形态管理接口**。

完整方案见：`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/.loom-drafts/cloud-skills-mcp-design.md`

## 当前状态（已经做好的，不要重做）

✅ 现有云 skill 包（**直接复用，不要重写**）：
- `~/.claude/skills/tencent-cloud/` — 完整 5 脚本 (cvm/lighthouse/cdb/cos/cloudbase) + 真实跑通 lhins-prrbaecg Lighthouse 实例
- `~/.claude/skills/alicloud/` — clone 自 `cinience/alicloud-skills` 184 sub-skill (Apache 2.0)
- `~/.claude/skills/google-cloud/` — clone 自 `google/skills` 99 sub-skill (Apache 2.0)

✅ 关键修复（已记入 memory）：
- tccli env var 必须 `TENCENTCLOUD_SECRET_ID`（带下划线）
- macOS BSD date 用 `-r @timestamp`，LibreSSL 用 `awk '{print $NF}'`
- bash pipe 不用 `if cmd | head`，用 mktemp + 同步调用 + 真 exit code

## 本次目标（你这次会话要做完的）

按 `.loom-drafts/cloud-skills-mcp-design.md` 的 **Phase 1 (30 分钟)** 执行：

### 1. 写共享 SDK (`internal/mcp/sdk/`) — ~200 行 Go

文件路径：`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/internal/mcp/sdk/`

需要的文件：
- `creds.go` — 凭证抽象 (Keychain + env + 各云 CLI 配置文件)
  - 公共 API: `LoadCreds(cloudName string) (*Creds, error)`
  - 6 个云的支持: aws / azure / gcp / alicloud / tencent-cloud / baiducloud
  - Keychain service 名映射表 (见设计文档 §4)
  - 6 套 env var 读取 (AWS_*, AZURE_*, GOOGLE_APPLICATION_CREDENTIALS, ALIBABACLOUD_*, TENCENTCLOUD_*, BCE_*)
  - 默认 region 表 (见设计文档 §4)
- `tools.go` — 工具注册辅助
  - 用 mcp-go SDK (github.com/mark3labs/mcp-go)
  - 工具定义: name / description / JSON schema 输入
- `errors.go` — 错误处理
  - 把云 SDK 错误映射到 MCP error code
  - 危险操作（destroy/terminate）必须 `--force` 二次确认逻辑
- `apidoc.go` — OpenAPI 抓取框架 (Phase 1 只要骨架)

### 2. 写 PoC MCP server (`internal/mcp/tencent/main.go`) — ~300 行 Go

文件路径：`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/internal/mcp/tencent/main.go`

要求：
- 用 mcp-go SDK 启动 stdio MCP server
- 实现 4 个工具 (Phase 1 起步 4 个，验证架构):
  1. `tencent_cvm_list_instances` — 调 tccli cvm DescribeInstances
  2. `tencent_cvm_describe_instance` — 调 tccli cvm DescribeInstances --InstanceIds.0
  3. `tencent_cvm_start_instance` — 调 tccli cvm StartInstances (需要 --force 确认)
  4. `tencent_cvm_stop_instance` — 调 tccli cvm StopInstances (需要 --force 确认)
- 用 `sdk.LoadCreds("tencent-cloud")` 读凭证
- 用 `sdk.RegisterTool()` 注册工具
- `--help` 退出码 0
- stdin JSON-RPC 接收请求
- stdout JSON-RPC 返回响应

### 3. 编译 + 端到端测试

```bash
cd /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/

# 1. go mod init (如果没有)
go mod init github.com/loom/agent-platform 2>/dev/null || true

# 2. 加 mcp-go 依赖
go get github.com/mark3labs/mcp-go@latest

# 3. 编译
go build -o ~/bin/tencent-cloud-mcp ./internal/mcp/tencent/

# 4. 测试 --help
~/bin/tencent-cloud-mcp --help
# 期望: exit 0, 输出工具列表

# 5. 测试 JSON-RPC 调用 (用 mavis 用户真实 AKSK)
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ~/bin/tencent-cloud-mcp
# 期望: 返回 4 个工具定义

echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tencent_cvm_list_instances","arguments":{"region":"ap-shanghai","limit":10}}}' | ~/bin/tencent-cloud-mcp
# 期望: 返回 InstanceSet (即使 TotalCount=0 也算成功)

# 6. 旧 bash 脚本仍然 OK (fallback)
bash -n ~/.claude/skills/tencent-cloud/scripts/cvm.sh
bash -n ~/.claude/skills/tencent-cloud/scripts/lighthouse.sh
# 期望: 两个都 exit 0
```

### 4. 更新 `~/.claude/skills/tencent-cloud/SKILL.md`

加一段 "MCP 调用方式" 章节:
- 描述: 本 skill 同时支持 bash scripts (fallback) 和 MCP server (推荐)
- 给出 stdio 调用样例
- 写 `~/.claude/mcp_servers.json` 注册模板

### 5. 写 `~/.claude/mcp_servers.json`

```json
{
  "mcpServers": {
    "tencent-cloud": {
      "command": "/Users/lune/bin/tencent-cloud-mcp",
      "env": {}
    }
  }
}
```

(其他 5 个云先不注册，等 Phase 2)

### 6. 写 `deliverable.md` (放在 `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/.loom-drafts/cloud-mcp-poc-deliverable.md`)

格式参考 Loom 的 deliverable 模板，包含:
- 完成的工作 (文件列表 + 行数)
- 编译/测试输出 (实际跑出来的，不是 "应该 work")
- bash -n 输出 (实际 6 个 SKILL.md 的脚本都过)
- JSON-RPC 调用输出 (实际拿到的)
- 已知问题 (如果有)
- 下一步建议 (Phase 2 起步)

**最后一行必须是**: `VERDICT: PASS` 或 `VERDICT: FAIL`

## 硬规则

1. **VERDICT 必须有** — 跟 Plan engine 兼容, 缺失 = auto-reject
2. **测试要有实际输出** — 不是 "should work" / "compiled"，是真实跑出来的 exit code 和 stdout
3. **bash -n 必跑** — 6 个云 skill 的所有 bash 脚本都得过
4. **凭证零暴露** — 不在 deliverable / commit / log 里出现 AKSK
5. **mavis-trash 用 mavis-trash 不用 rm** — 任何删除走 mavis-trash
6. **trap 不用 single quote 包变量** — 用 `rm -f "${VAR}"` 立即清理
7. **pipe 不用 if** — 用 mktemp + 同步调用 + 真 exit code (这是 tencent SKILL.md 的 lessons learned)

## 不要做 (YAGNI)

- ❌ 不要写 6 个云全实现 — 只要 tencent PoC
- ❌ 不要写 Web UI
- ❌ 不要写跨云联邦查询
- ❌ 不要做 Terraform 兼容
- ❌ 不要做 SSO / OAuth
- ❌ 不要 touch 现有 6 个 SKILL.md 包的 scripts/ (除非必要兼容)
- ❌ 不要 push 到 remote (用户没让)

## 完成判据

1. ✅ `~/bin/tencent-cloud-mcp` 存在且能 `--help` exit 0
2. ✅ JSON-RPC `tools/list` 返回 4 个工具定义
3. ✅ JSON-RPC `tools/call` 调 `tencent_cvm_list_instances` 返回真实数据 (TotalCount: 0 也算)
4. ✅ `~/.claude/skills/tencent-cloud/SKILL.md` 已更新 MCP 章节
5. ✅ `~/.claude/mcp_servers.json` 已创建
6. ✅ `deliverable.md` 存在, 最后一行 `VERDICT: PASS`
7. ✅ 6 个云 skill 的所有 bash 脚本 `bash -n` 仍然 OK
8. ✅ 没碰 AKSK (deliverable 里用 "AKID***" 占位)

</GOAL>

---

## 关键提醒 (给 Mavis 看的)

1. **写代码前先 grep**: 任何新 API 名字 / error code / status 值, 进 prompt 前先 `grep -rn` Loom 实际代码
2. **避免长 worker context overflow**: 每完成一个文件立刻 commit, deliverable.md 在 report-back 前写好
3. **macOS 兼容性**: BSD date `-r @timestamp` 不用 `-d`, LibreSSL `awk '{print $NF}'` 不用 `'{print $2}'`, bash echo 默认不解释 `\n`
4. **凭证**: 用现有 `~/.claude/skills/tencent-cloud/scripts/_creds.sh` 的模式, Keychain service=tencent-cloud, env var TENCENTCLOUD_SECRET_ID / TENCENTCLOUD_SECRET_KEY (带下划线!)
5. **不要等用户**: 自己决定细节, 做完报告。如果遇到必须问的问题, 用 ask_user 工具

## 跨项目 lesson 速查

- **mavis-trash**: 用 mavis-trash 不用 rm
- **trap + single quote**: 会把 ${VAR} 当字面量
- **bash pipe + if**: 检查 last cmd exit code, head 总 0 → 永远走 then
- **2>&1 | jq**: tccli 错误被 pipe 走, jq 失败被 || 兜底
- **tccli env var**: TENCENTCLOUD_SECRET_ID (带下划线, 不是 SECRETID)
