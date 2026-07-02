#!/usr/bin/env python3
"""
真机验证控制台 UI（Playwright）—— 用真浏览器加载控制台、断言关键面板都渲染出来、
抓 JS 控制台错误、存整页截图。开发 Loom 的 agent 用它做 UI 改动的"真机验证"门，
不再只靠 node --check（语法）+ curl（接口）盲改。

先装一次：  pip install playwright  &&  playwright install chromium
跑：        python3 scripts/verify_console.py [URL]   （默认 http://localhost:8899）
或：        ./loom verify-ui
退出码 0=PASS，非 0=FAIL（可进 CI / 当 Eval Gate）。
"""
import sys

URL = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8899"
# 标题断言（只验面板渲染，不验行内容——行需点刷新才探测，合同禁止自动探测）
WANT = ["角色 → 载体 → 后端", "编辑角色流水线", "快速咨询", "后端真活性", "额度薅羊毛", "模型评分"]
SHOT = "/tmp/loom_console.png"

try:
    from playwright.sync_api import sync_playwright
except Exception:  # noqa: BLE001
    print("✗ 未装 playwright。先： pip install playwright && playwright install chromium")
    sys.exit(2)

errors, reqs = [], []
with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page()
    page.on("console", lambda m: errors.append(m.text) if m.type == "error" else None)
    page.on("pageerror", lambda e: errors.append(str(e)))
    page.on("request", lambda r: reqs.append(r.url))
    page.goto(URL, wait_until="networkidle", timeout=20000)
    page.wait_for_timeout(1500)            # 让前端拉 /api/* 渲染面板
    body = page.inner_text("body")
    page.screenshot(path=SHOT, full_page=True)
    browser.close()

missing = [w for w in WANT if w not in body]
# 计费面板必须懒加载：开页面/轮询都不许自动打 /api/liveness（每次=5 次计费推理）
auto_billable = [u for u in reqs if "/api/liveness" in u]
print(f"URL: {URL}")
print(f"面板: {'全部渲染 ✓' if not missing else '缺 ' + str(missing)}")
print(f"JS 错误: {'无 ✓' if not errors else errors[:3]}")
print(f"开页自动探测 /api/liveness: {'无（懒加载 ✓）' if not auto_billable else '有！' + str(len(auto_billable)) + ' 次（违反零计费规则）'}")
print(f"截图: {SHOT}")
ok = not missing and not errors and not auto_billable
print("RESULT:", "PASS" if ok else "FAIL")
sys.exit(0 if ok else 1)
