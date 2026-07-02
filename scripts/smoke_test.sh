#!/usr/bin/env bash
# 验证本地网关：1) 正常调用  2) 主模型坏掉时自动降级
set -e
GW=${GW:-http://localhost:4000}
KEY=${KEY:-sk-local-demo}

call () {
  curl -s "$GW/v1/chat/completions" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
    -d "{\"model\":\"$1\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" \
    | python3 -c "import sys,json;print(json.load(sys.stdin)['choices'][0]['message']['content'])"
}

echo "1) 正常主力模型 claude-sonnet:"
call claude-sonnet

echo
echo "2) 故意调坏掉的 glm-flagship（应自动降级到 gpt-flagship）:"
call glm-flagship
