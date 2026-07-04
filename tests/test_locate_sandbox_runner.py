# L1 contract harness — 不落真实仓库，仅供 devkit 回路校验
import os
import sys
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path

# 下方 SCRIPT 是被测对象的源码字符串（与最终实现完全一致）
import sys
sys.path.insert(0, "scripts")
from debug.locate_sandbox_runner_script import SCRIPT  # 也可直接 inline

REPO_ROOT = Path(__file__).resolve().parent.parent  # devkit/ 下时取上一层

def run_script(cwd: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, '-c', SCRIPT],
        cwd=str(cwd),
        capture_output=True,
        text=True,
    )

class LocateSandboxRunnerContract(unittest.TestCase):

    def test_happy_path_when_running_inside_repo_root(self):
        """旗舰用例：在真实仓库根下执行，日志满足 acceptance。"""
        # 先把仓库根下一个已知存在的 candidate 文件当作 rglob 的检测锚点
        # 本用例不假设 build/，只验证行为契约
        proc = run_script(REPO_ROOT)
        log_path = REPO_ROOT / 'devkit' / 'runs' / 'locate-sandbox-runner.log'

        self.assertEqual(proc.returncode, 0, msg=proc.stderr)
        self.assertTrue(log_path.exists(), 'log file must be created')

        content = log_path.read_text(encoding='utf-8')

        # 验收：三种满足其一即可
        self.assertTrue(
            ('NO_MATCH' in content)
            or ('verify_command' in content)
            or any(line.startswith('HIT ') for line in content.splitlines()),
            'log must contain NO_MATCH, verify_command, or at least one HIT line',
        )

        # stdout 必须给出最终 log path
        self.assertIn('devkit/runs/locate-sandbox-runner.log', proc.stdout)

    def test_no_match_returns_zero_with_NO_MATCH_marker(self):
        """Non-happy-path：cwd 内无 sandbox_runner*.py 时，应写 NO_MATCH 且返回 0。"""
        with tempfile.TemporaryDirectory() as tmp:
            tmp_root = Path(tmp)

            # 准备保证匹配为空的目录：建几个无关 .py
            (tmp_root / 'foo.py').write_text('# nope\n')
            (tmp_root / 'sub').mkdir()
            (tmp_root / 'sub' / 'bar.py').write_text('# nope\n')

            # 注意：脚本逻辑会尝试写 ./devkit/runs/...log（相对 cwd）
            # 为了让契约可观察，脚本本身保证存在性 / 写盘行为
            proc = run_script(tmp_root)
            log_path = tmp_root / 'devkit' / 'runs' / 'locate-sandbox-runner.log'

            self.assertEqual(proc.returncode, 0, msg=proc.stderr)
            self.assertTrue(log_path.exists())
            self.assertIn('NO_MATCH', log_path.read_text(encoding='utf-8'))

    def test_extracts_verify_command_lines_with_line_numbers(self):
        """若首个匹配存在且包含 verify_command，应输出所在行号。"""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            target = root / 'sandbox_runner_dev.py'
            target.write_text(
                'def run():\n'
                '    cmd = "echo hi"\n'
                '    # verify_command: shellcheck this\n'
                '    return cmd\n'
                '    verify_command_alt = True\n',  # 第二匹配
                encoding='utf-8',
            )

            proc = run_script(root)
            log_path = root / 'devkit' / 'runs' / 'locate-sandbox-runner.log'

            self.assertEqual(proc.returncode, 0, msg=proc.stderr)
            content = log_path.read_text(encoding='utf-8')

            # 不应出现 NO_MATCH
            self.assertNotIn('NO_MATCH', content)

            # 至少含两条 verify_command 命中行
            hits = [
                ln for ln in content.splitlines()
                if 'verify_command' in ln and ln.startswith('VCMD ')
            ]
            self.assertGreaterEqual(len(hits), 2, msg=content)

if __name__ == '__main__':
    unittest.main(verbosity=2)
