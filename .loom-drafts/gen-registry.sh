#!/bin/bash
# 给其他 5 个云批量生成 server.json 模板
# macOS bash 3.2 兼容版 (不用 declare -A)

set -e

SCHEMA="https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json"
VERSION="0.2.0"
REPO="tttboy123/cloud-skills-mcp"

# 格式: cloud|en_cn|pkg|cred_envs_json|tool_prefix
ENTRIES=(
  "alicloud|Aliyun ECS / 阿里云 ECS|alicloud-ecs|[{\"name\":\"ALIBABACLOUD_ACCESS_KEY_ID\",\"isSecret\":true},{\"name\":\"ALIBABACLOUD_ACCESS_KEY_SECRET\",\"isSecret\":true}]|alicloud_ecs"
  "aws|AWS EC2 / 亚马逊云 EC2|aws-ec2|[{\"name\":\"AWS_ACCESS_KEY_ID\",\"isSecret\":true},{\"name\":\"AWS_SECRET_ACCESS_KEY\",\"isSecret\":true}]|aws_ec2"
  "azure|Azure Virtual Machine / 微软云 VM|azure-vm|[{\"name\":\"AZURE_TENANT_ID\",\"isSecret\":false},{\"name\":\"AZURE_CLIENT_SECRET\",\"isSecret\":true},{\"name\":\"AZURE_SUBSCRIPTION_ID\",\"isSecret\":false}]|azure_vm"
  "google-cloud|Google Compute Engine / GCP GCE|gce|[{\"name\":\"GOOGLE_APPLICATION_CREDENTIALS\",\"isSecret\":true,\"format\":\"filepath\"}]|google_compute"
  "baiducloud|Baidu Cloud Compute (BCC) / 百度智能云 BCC|bcc|[{\"name\":\"BCE_ACCESS_KEY_ID\",\"isSecret\":true},{\"name\":\"BCE_SECRET_ACCESS_KEY\",\"isSecret\":true}]|baiducloud_bcc"
)

for entry in "${ENTRIES[@]}"; do
  IFS='|' read -r cloud en_cn pkg cred_envs tool_prefix <<< "$entry"

  cat > "registry/${cloud}/server.json" <<EOF
{
  "\$schema": "${SCHEMA}",
  "name": "io.github.${REPO}/${cloud}",
  "title": "${en_cn} MCP Server",
  "description": "MCP server for ${en_cn}. 通过 native CLI 调云 API, 凭证走 3-tier (macOS Keychain → env → CLI config). 1 键安装: curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | bash -s -- --cloud=${cloud}",
  "version": "${VERSION}",
  "homepage": "https://github.com/${REPO}",
  "repository": {"url": "https://github.com/${REPO}", "source": "git"},
  "license": "MIT",
  "icons": [{"src": "https://raw.githubusercontent.com/${REPO}/main/docs/icon-${cloud}.svg", "mimeType": "image/svg+xml", "sizes": ["any"]}],
  "packages": [
    {"registryType": "oci", "identifier": "ghcr.io/${REPO}/${cloud}:${VERSION}", "transport": {"type": "stdio"}},
    {"registryType": "npm", "identifier": "@tttboy123/cloud-skills-mcp-${pkg}", "version": "${VERSION}", "transport": {"type": "stdio"}}
  ],
  "environmentVariables": ${cred_envs},
  "tools": [
    {"name": "${tool_prefix}_list_instances", "description": "List ${en_cn} instances in a region. 列出 ${en_cn} 资源."},
    {"name": "${tool_prefix}_describe_instance", "description": "Describe a single ${en_cn} instance by id. 按 ID 查 ${en_cn} 详情."},
    {"name": "${tool_prefix}_start_instance", "description": "START a ${en_cn} instance. 启动实例. Mutating — pass force=true to confirm."},
    {"name": "${tool_prefix}_stop_instance", "description": "STOP a ${en_cn} instance. 关机实例. Mutating — pass force=true to confirm."}
  ]
}
EOF
  echo "✓ registry/${cloud}/server.json"
done

echo ""
echo "✅ 5 个 server.json 全部生成"
ls registry/*/server.json
