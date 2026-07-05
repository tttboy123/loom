"""
CLI 入口。三种用法：

  # 1) 跑一遍 Loom R&D Loop（默认 autonomous；可显式切 report-only）
  python -m devkit "为 Buddys 实现 capture：把一句话解析成结构化 inventory items"
  python -m devkit --task-file task.md --stages brainstorm,plan,implement

  # 2) @ask-model：临时问一个/多个载体一句（不开整条 loop；多个则并行对比）
  python -m devkit ask "用一句话讲依赖倒置" --models deepseek,glm,loom-reviewer

  # 3) diff：对比两次运行的 build/ 产物（默认对比上一次带 build 的运行）
  python -m devkit diff 20260623-1730 --against 20260623-1700
"""
import argparse
import json
import os
import pathlib
import sys

from devkit.delivery_mode import DEFAULT_DELIVERY_MODE
from devkit.delivery_mode import VALID_DELIVERY_MODES
from devkit.model_aliases import normalize_model_name
from devkit.rdloop import run_loop


def _load_backlog_payload(path: pathlib.Path) -> tuple[list[dict], bool]:
    path = pathlib.Path(path)
    if not path.exists():
        return [], False
    data = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(data, dict) and isinstance(data.get("tasks"), list):
        return [dict(item) for item in data["tasks"] if isinstance(item, dict)], True
    if isinstance(data, list):
        return [dict(item) for item in data if isinstance(item, dict)], False
    return [], False


def _atomic_write_text(path: pathlib.Path, text: str) -> None:
    path = pathlib.Path(path)
    tmp = path.with_name(f".{path.name}.tmp")
    with tmp.open("w", encoding="utf-8") as fp:
        fp.write(text)
        fp.flush()
        os.fsync(fp.fileno())
    os.replace(tmp, path)


def _merge_backlog_state(candidate: list[dict], current: list[dict]) -> list[dict]:
    """Merge a stale in-memory backlog with latest on-disk state.

    Candidate order wins. New tasks already written by another writer are
    appended instead of being lost, and missing fields on candidate items are
    backfilled from disk.
    """
    merged: list[dict] = []
    current_by_id = {
        str(item.get("id", "")).strip(): dict(item)
        for item in current
        if isinstance(item, dict) and str(item.get("id", "")).strip()
    }
    seen_ids: set[str] = set()

    for item in candidate:
        if not isinstance(item, dict):
            continue
        merged_item = dict(item)
        task_id = str(merged_item.get("id", "")).strip()
        if task_id and task_id in current_by_id:
            disk_item = current_by_id[task_id]
            for key, value in disk_item.items():
                merged_item.setdefault(key, value)
            seen_ids.add(task_id)
        merged.append(merged_item)

    for item in current:
        if not isinstance(item, dict):
            continue
        task_id = str(item.get("id", "")).strip()
        if task_id and task_id in seen_ids:
            continue
        merged.append(dict(item))
    return merged


def _write_backlog(path: pathlib.Path, backlog: list[dict]) -> list[dict]:
    path = pathlib.Path(path)
    current, wrapped = _load_backlog_payload(path)
    merged = _merge_backlog_state(backlog, current)
    payload = {"tasks": merged} if wrapped else merged
    _atomic_write_text(path, json.dumps(payload, ensure_ascii=False, indent=2) + "\n")
    return merged


def _reclaim_stale_running(path: pathlib.Path, backlog: list[dict]) -> list[dict]:
    from devkit import lease as _lease

    reclaimed = _lease.reclaim_stale_running(
        backlog,
        current_owner_pid=os.getpid(),
    )
    if reclaimed["reclaimed"]:
        return _write_backlog(path, reclaimed["backlog"])
    return reclaimed["backlog"]


def _prune_stale_pending(path: pathlib.Path, backlog: list[dict]) -> list[dict]:
    from devkit import autoloop as _autoloop

    pruned = _autoloop.prune_stale_pending(backlog)
    if pruned["stopped"]:
        return _write_backlog(path, pruned["backlog"])
    return pruned["backlog"]


def _prune_human_only_pending(path: pathlib.Path, backlog: list[dict]) -> list[dict]:
    from devkit import autoloop as _autoloop

    pruned = _autoloop.prune_human_only_pending(backlog)
    if pruned["stopped"]:
        return _write_backlog(path, pruned["backlog"])
    return pruned["backlog"]


def main() -> int:
    argv = sys.argv[1:]
    if argv and argv[0] == "ask":
        return _cmd_ask(argv[1:])
    if argv and argv[0] == "diff":
        return _cmd_diff(argv[1:])
    if argv and argv[0] == "roles":
        return _cmd_roles(argv[1:])
    if argv and argv[0] == "quota":
        return _cmd_quota(argv[1:])
    if argv and argv[0] == "scores":
        return _cmd_scores(argv[1:])
    if argv and argv[0] == "stages":
        return _cmd_stages(argv[1:])
    if argv and argv[0] == "rate":
        return _cmd_rate(argv[1:])
    if argv and argv[0] == "backlog":
        return _cmd_backlog(argv[1:])
    if argv and argv[0] == "feature":
        return _cmd_feature(argv[1:])
    if argv and argv[0] == "recipes":
        return _cmd_recipes(argv[1:])
    if argv and argv[0] == "fitness":
        return _cmd_fitness(argv[1:])
    if argv and argv[0] == "task":
        return _cmd_task(argv[1:])
    if argv and argv[0] == "asset":
        return _cmd_asset(argv[1:])
    if argv and argv[0] == "safety":
        return _cmd_safety(argv[1:])
    if argv and argv[0] == "recommend":
        return _cmd_recommend(argv[1:])
    if argv and argv[0] == "runs":
        return _cmd_runs(argv[1:])
    if argv and argv[0] == "learn":
        return _cmd_learn(argv[1:])
    if argv and argv[0] == "init":
        return _cmd_init(argv[1:])
    if argv and argv[0] == "config":
        return _cmd_config(argv[1:])
    if argv and argv[0] == "radar":
        return _cmd_radar(argv[1:])
    if argv and argv[0] == "migrate":
        return _cmd_migrate(argv[1:])
    if argv and argv[0] == "status":
        from devkit.dashboard import render
        print(render())
        return 0
    if argv and argv[0] == "graph":
        import json as _json
        _bl_path = pathlib.Path("devkit/backlog.json")
        _bl = _json.loads(_bl_path.read_text()) if _bl_path.exists() else []
        from devkit.graph_cli import ascii_tree, summary_line
        print(summary_line(_bl))
        print(ascii_tree(_bl))
        return 0
    if argv and argv[0] == "auto":
        return _cmd_auto(argv[1:])
    if argv and argv[0] == "iterate":
        return _cmd_iterate(argv[1:])
    if argv and argv[0] == "decisions":
        return _cmd_decisions(argv[1:])
    if argv and argv[0] == "bench":
        return _cmd_bench(argv[1:])
    if argv and argv[0] == "setup":
        return _cmd_setup(argv[1:])
    return _cmd_run(argv)            # 默认：跑研发流程（保持 `devkit "任务"` 原样）


def _locate_project(arg, dir_opt):
    from devkit import backlog as B
    if dir_opt:
        return pathlib.Path(dir_opt)
    if arg:
        return B.project_dir(arg)
    if B.PROJECTS.exists():
        dirs = [d for d in B.PROJECTS.iterdir() if (d / "backlog.json").is_file()]
        if len(dirs) == 1:
            return dirs[0]
        if dirs:
            raise SystemExit("有多个项目，请用 --dir 指定：" + ", ".join(d.name for d in dirs))
    raise SystemExit('没有项目，先 `devkit backlog "应用想法"` 初始化')


def _cmd_run(argv) -> int:
    from devkit.roles import load_stages, active_source
    try:
        all_stages = load_stages()           # 用户自定义角色优先，无文件则内置默认
    except Exception as e:                    # noqa: BLE001
        raise SystemExit(f"角色配置加载失败：{e}（用 `devkit roles path` 看用的哪个文件）")
    by_key = {s.key: s for s in all_stages}
    p = argparse.ArgumentParser(
        prog="devkit", description="Loom R&D Loop 开发流程套件",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="另有子命令：\n"
               "  devkit ask <prompt> --models deepseek,glm   临时问一个/多个载体（并行对比）\n"
               "  devkit diff <ts> [--against <ts>]           对比两次运行的 build/ 产物\n"
               "  devkit roles init|list|path                 定义/查看你自己的角色流水线\n"
               "  devkit quota                                额度薅羊毛看板（优先用免费/订阅）\n"
               "  devkit scores                               模型评分（实际使用 + 官网评测）\n"
               "  devkit stages                               按阶段透视成本/tokens（钱花在哪个阶段）\n"
               "  devkit rate <后端> up|down                  记一次实际体验（计入综合分）\n"
               "  devkit backlog \"<应用想法>\"                  Initializer：拆成特性清单（增量构建）\n"
               "  devkit feature [--count K]                  增量构建下一个/K 个特性，测试绿才标 done\n"
               "  devkit recipes                              列出内置管道预设（--recipe NAME 直接使用）\n"
               "  devkit fitness                              模型适配度：按任务类型分桶看哪个模型最擅长哪类任务\n"
               "  devkit runs [<run-id>]                      任务证据链：列表/详情查看历史 runs\n"
               "  （各子命令 -h 看帮助）")
    p.add_argument("task", nargs="?", help="开发任务描述")
    p.add_argument("--task-file", help="从文件读取任务（覆盖位置参数）")
    p.add_argument("--stages", help="逗号分隔的阶段子集，默认全跑："
                                    + ",".join(s.key for s in all_stages))
    p.add_argument("--base-url", default=os.environ.get("LITELLM_BASE_URL", "http://localhost:4000"),
                   help="LiteLLM 网关地址（默认读 LITELLM_BASE_URL）")
    p.add_argument("--max-tokens", type=int, default=900)
    p.add_argument("--carrier", action="append", default=[], metavar="STAGE=MODEL",
                   help="本次运行临时把某阶段换成别的载体，可重复，如 --carrier review=codex-sub")
    p.add_argument("--executor", action="append", default=[], metavar="STAGE=NAME",
                   help="某阶段用哪个执行器：chat|hermes|openclaw，可重复（可同时嵌入），"
                        "如 --executor implement=hermes --executor review=openclaw")
    p.add_argument("--apply", metavar="DIR",
                   help="人类门：把 implement 的物化产物（仅测试通过时）复制到 DIR")
    p.add_argument("--apply-git", metavar="REPO",
                   help="人类门：测试通过时在 REPO 新建分支并 commit 产物（不 push）")
    p.add_argument("--delivery-mode", default=DEFAULT_DELIVERY_MODE, choices=sorted(VALID_DELIVERY_MODES),
                   help="交付模式：autonomous|report-only|apply-required|apply-git（默认 autonomous）")
    p.add_argument("--branch", metavar="NAME", help="--apply-git 的分支名（默认 loom/<时间戳>）")
    p.add_argument("--run-id", metavar="TS", help="指定运行 id/时间戳（控制台用于实时跟踪）")
    p.add_argument("--golden", metavar="FILE", help="Eval Gate：golden 用例 json，跑在 build 上，不过即 NO-GO")
    p.add_argument("--sandbox", action="store_true", help="OS 级沙箱跑 agentic 执行器（写盘限沙箱+状态目录，macOS sandbox-exec）")
    p.add_argument("--compact-model", default="deepseek", metavar="MODEL",
                   help="上下文压缩用的便宜模型（默认 deepseek）；长上游产物先压成要点再喂下游")
    p.add_argument("--no-compact", action="store_true", help="关闭上下文压缩，回到截断模式")
    p.add_argument("--no-cache", action="store_true",
                   help="关闭精确响应缓存（默认开；相同请求免费命中、不重复计费）")
    p.add_argument("--budget", type=float, metavar="USD",
                   help="软预算护栏：本次运行累计花费超过该美元数则停掉剩余阶段并 NO-GO")
    p.add_argument("--iterate", type=int, default=0, metavar="N",
                   help="迭代循环（借鉴 Anthropic）：评判 NO-GO 就回灌构建者修复并重测重评，最多 N 轮直到通过")
    p.add_argument("--contract", type=int, default=0, metavar="N",
                   help="Sprint Contract：implement 前评判者先约定 ~N 条可测验收点（golden），注入构建者并作为 Eval Gate")
    p.add_argument("--contract-rounds", type=int, default=0, metavar="N",
                   help="合同协商：评判者拟好后，构建者再 N 轮收紧/修正（带反削弱地板）；0=一次性（默认）")
    p.add_argument("--cascade", metavar="c1,c2,...",
                   help="级联升级（FrugalGPT）：implement 用 cheap→strong 载体阶梯，初始用最便宜的，"
                        "评判 NO-GO 才升级下一档；蕴含 iterate。与 --carrier implement=X 互斥")
    p.add_argument("--recipe", metavar="NAME",
                   help="管道预设（devkit recipes 看列表）：一次性指定 stages + carriers 默认值，"
                        "可被 --stages / --carrier / --cascade 覆盖。例：--recipe cheap-dev")
    p.add_argument("--blind-review", action="store_true", dest="blind_review",
                   help="T15 盲审：review 阶段不接收实现代码，只看任务规格 + 测试输出，防止评审者顺着实现走")
    p.add_argument("--verify", action="store_true", dest="physical_verify",
                   help="T16 物理验证：在独立子进程里 smoke-import 所有模块并交叉比对 golden 用例")
    p.add_argument("--ponytail", action="store_true",
                   help="PonyTail 过工程审查门：把 review 阶段的 system prompt 替换成"
                        "【最小 diff / 零新依赖 / 无多余抽象】检查契约，REQUEST-CHANGES 则触发迭代")
    p.add_argument("--safety", action="store_true",
                   help="跑完后自动对 build/ 做安全扫描（检查硬编码密钥 / shell 注入等）")
    p.add_argument("--auto-carrier", action="store_true",
                   help="根据历史 fitness 自动为 implement 选最优载体（devkit recommend 同源）")
    p.add_argument("--inject-asset", action="append", default=[], metavar="NAME",
                   help="把资产内容注入到任务前缀（可重复），如 --inject-asset anti-overengineering")
    p.add_argument("--no-unverified-assets", action="store_true",
                   help="拒绝注入 trust_level=0（未审查）的资产，hard NO-GO")
    p.add_argument("--health-probe", action="store_true", dest="health_probe",
                   help="运行前对所有 carrier 发起实时健康探针，结果写 carrier_health.json 并更新路由")
    p.add_argument("--task-id", metavar="SLUG",
                   help="把本次 run 自动关联到已有任务（devkit task new 创建，devkit task list 查看）")

    # Load loom.toml project config as defaults (CLI flags always override)
    from devkit import config as _cfg
    _conf = _cfg.load_config()
    if _conf:
        _sd: dict = {}
        if "stages" in _conf:       _sd["stages"] = _conf["stages"]
        if "cascade" in _conf:      _sd["cascade"] = _conf["cascade"]
        if "max_tokens" in _conf:   _sd["max_tokens"] = int(_conf["max_tokens"])
        if "recipe" in _conf:       _sd["recipe"] = _conf["recipe"]
        if "iterate" in _conf:      _sd["iterate"] = int(_conf["iterate"])
        if "budget" in _conf:       _sd["budget"] = float(_conf["budget"])
        if "delivery_mode" in _conf: _sd["delivery_mode"] = str(_conf["delivery_mode"])
        if "compact_model" in _conf: _sd["compact_model"] = _conf["compact_model"]
        if "base_url" in _conf:     _sd["base_url"] = _conf["base_url"]
        if "safety" in _conf:       _sd["safety"] = bool(_conf["safety"])
        if "auto_carrier" in _conf: _sd["auto_carrier"] = bool(_conf["auto_carrier"])
        if "ponytail" in _conf:     _sd["ponytail"] = bool(_conf["ponytail"])
        if "no_compact" in _conf:   _sd["no_compact"] = bool(_conf["no_compact"])
        if "no_cache" in _conf:     _sd["no_cache"] = bool(_conf["no_cache"])
        if _sd:
            p.set_defaults(**_sd)

    args = p.parse_args(argv)

    # Inject carrier defaults from loom.toml [carrier] section (before explicit --carrier)
    if _conf.get("_carrier"):
        _explicit = {item.split("=")[0].strip() for item in args.carrier}
        for _entry in _conf["_carrier"]:
            _stage = _entry.split("=")[0].strip()
            if _stage not in _explicit:
                args.carrier.append(_entry)

    # --recipe 展开：先把预设的 stages/carriers 作为默认值，再让显式参数覆盖
    if args.recipe:
        from devkit.recipes import get_recipe as _get_recipe
        try:
            _recipe = _get_recipe(args.recipe)
        except KeyError:
            from devkit.recipes import list_recipes as _lr
            p.error(f"未知 recipe {args.recipe!r}；可选：{_lr()}")
        if args.stages is None:
            args.stages = ",".join(_recipe["stages"])
        # 仅把 recipe 里的 carriers 注入未被显式 --carrier 覆盖的阶段
        _explicit_stages = {item.split("=")[0].strip() for item in args.carrier}
        for _stage, _car in _recipe.get("carriers", {}).items():
            if _stage not in _explicit_stages:
                args.carrier.append(f"{_stage}={_car}")

    def _kv(items, flag, valid=None):
        m = {}
        for item in items:
            if "=" not in item:
                p.error(f"{flag} 需写成 STAGE=VALUE，收到：{item}")
            k, v = item.split("=", 1)
            k, v = k.strip(), v.strip()
            if k not in by_key:
                p.error(f"未知阶段 {k!r}；可选：{list(by_key)}")
            if valid and v not in valid:
                p.error(f"{flag} 值非法 {v!r}；可选：{valid}")
            if flag == "--carrier":
                v = normalize_model_name(v, stage=k)
            m[k] = v
        return m

    overrides = _kv(args.carrier, "--carrier")
    executor_map = _kv(args.executor, "--executor", valid={"chat", "hermes", "openclaw", "codex", "opencode"})

    if getattr(args, "inject_asset", None):
        from devkit import asset as _asset
        _no_unverified = getattr(args, "no_unverified_assets", False)
        prefixes = []
        for _name in args.inject_asset:
            _a = _asset.get_asset(_name)
            if _a is None:
                p.error(f"找不到资产：{_name!r}（用 `devkit asset list` 查看可用资产）")
            _trust = _a.get("trust_level", 0)
            if _trust == 0:
                if _no_unverified:
                    print(f"🚫 --no-unverified-assets: 资产 {_name!r} trust_level=0（未审查），已拒绝注入。"
                          f"\n   用 `devkit asset trust {_name} 1` 审查后再注入。")
                    return 1
                print(f"⚠️  资产 {_name!r} trust_level=0（未审查），建议用 `devkit asset trust {_name} 1` 审查。")
            prefixes.append(f"[约束 {_a['name']} trust=L{_trust}]\n{_a['content']}")
        if prefixes:
            task = "\n\n".join(prefixes) + "\n\n---\n\n" + task

    if getattr(args, "auto_carrier", False) and "implement" not in overrides:
        from devkit import insight as _ins
        _rec = _ins.recommend_model(task)
        if _rec["backend"] is not None:
            overrides["implement"] = _rec["backend"]
            print(f"[auto-carrier] implement={_rec['backend']} "
                  f"(task_type={_rec['task_type']}, ok_rate={_rec['ok_rate']}%)")

    cascade = [c.strip() for c in (args.cascade or "").split(",") if c.strip()]
    if args.cascade is not None and not cascade:
        p.error("--cascade 不能为空，需逗号分隔的载体阶梯（cheap→strong）")
    if cascade and "implement" in overrides:
        p.error("--cascade 与 --carrier implement=… 冲突：cascade 已接管 implement 载体阶梯")

    task = pathlib.Path(args.task_file).read_text().strip() if args.task_file else args.task
    if not task:
        p.error("缺少任务：给位置参数或 --task-file")

    if args.no_cache:                     # 进程级关掉精确响应缓存
        import devkit.rdloop as _rd
        _rd.CACHE_ENABLED = False

    stages = all_stages
    if args.stages:
        keys = [k.strip() for k in args.stages.split(",") if k.strip()]
        bad = [k for k in keys if k not in by_key]
        if bad:
            p.error(f"未知阶段 {bad}；可选：{list(by_key)}")
        stages = [by_key[k] for k in keys]

    if getattr(args, "ponytail", False):
        from devkit import ponytail as _ponytail
        stages = _ponytail.apply(stages)

    res = run_loop(task, stages=stages, base_url=args.base_url,
                   max_tokens=args.max_tokens, carrier_overrides=overrides,
                   executor_map=executor_map, delivery_mode=args.delivery_mode, apply_target=args.apply,
                   apply_git=args.apply_git, apply_branch=args.branch, run_id=args.run_id,
                   golden=args.golden, os_sandbox=args.sandbox,
                   compact_model=(None if args.no_compact else args.compact_model),
                   budget=args.budget, iterate=args.iterate, contract=args.contract,
                   contract_rounds=args.contract_rounds, cascade=cascade,
                   blind_review=getattr(args, "blind_review", False),
                   physical_verify=getattr(args, "physical_verify", False),
                   health_probe=getattr(args, "health_probe", False))
    _safety_blocked = False
    if getattr(args, "safety", False):
        from devkit import safety as _safety
        _run_dir = pathlib.Path(res.get("run_dir", ""))
        if _run_dir.is_dir():
            _build = _run_dir / "build"
            if _build.is_dir():
                _sr = _safety.scan_build(_build)
                if not _sr["ok"]:
                    _errors = [v for v in _sr["violations"] if v.get("severity") == "error"]
                    _warns = [v for v in _sr["violations"] if v.get("severity") != "error"]
                    if _errors:
                        print(f"\n🚫 Safety NO-GO: {len(_errors)} error 级违规"
                              f"（devkit safety {_run_dir.name} 查详情）")
                        _safety_blocked = True
                    elif _warns:
                        print(f"\n⚠️  Safety: {len(_warns)} warn 级违规"
                              f"（devkit safety {_run_dir.name} 查详情）")
    if getattr(args, "task_id", None):
        _run_dir_name = pathlib.Path(res.get("run_dir", "")).name
        if _run_dir_name:
            from devkit import task_center as _TC
            try:
                _TC.link_run(args.task_id, _run_dir_name)
                print(f"[task] run {_run_dir_name} → task {args.task_id}")
            except KeyError:
                print(f"[task] 警告：找不到任务 {args.task_id!r}，跳过关联（devkit task list 查看可用任务）")
    return 1 if (res["blocked"] or _safety_blocked) else 0


def _cmd_ask(argv) -> int:
    """@ask-model：临时问一个/多个载体一句（多个则并行对比），与控制台共用 devkit.ask。"""
    from devkit.ask import ask_models
    from devkit.rdloop import load_master_key
    p = argparse.ArgumentParser(prog="devkit ask",
                                description="临时问一个/多个载体一句，不开整条 loop")
    p.add_argument("prompt", nargs="?", help="要问的问题")
    p.add_argument("--models", "-m", default="deepseek",
                   help="逗号分隔的载体/后端；多个则并行对比（默认 deepseek）")
    p.add_argument("--base-url", default=os.environ.get("LITELLM_BASE_URL", "http://localhost:4000"))
    p.add_argument("--max-tokens", type=int, default=700)
    a = p.parse_args(argv)
    if not a.prompt or not a.prompt.strip():
        p.error("缺少问题，如：devkit ask \"用一句话讲依赖倒置\" --models deepseek,glm")
    key = load_master_key()
    if not key:
        raise SystemExit("找不到 LITELLM_MASTER_KEY（设环境变量或填 agent-platform/.env）")
    models = [m.strip() for m in a.models.split(",") if m.strip()]
    results = ask_models(models, a.prompt, a.base_url, key, max_tokens=a.max_tokens)
    for r in results:
        if r["ok"]:
            print(f"\n\033[1m── {r['model']}\033[0m  "
                  f"(served={r['served']}, {r['tokens']}tok ${r['cost']:.5f})\n")
            print(r["content"])
        else:
            print(f"\n\033[1m── {r['model']}\033[0m  \033[31m✗ {r['error']}\033[0m")
    if len(results) > 1:
        print(f"\n合计：{sum(r.get('tokens',0) for r in results)} tok · "
              f"${sum(r.get('cost',0.0) for r in results):.5f}")
    return 0 if any(r["ok"] for r in results) else 1


def _cmd_roles(argv) -> int:
    """定义/查看你自己的角色流水线（角色×载体不再写死，来自用户配置文件）。"""
    from devkit import roles as R
    p = argparse.ArgumentParser(prog="devkit roles",
                                description="管理你自定义的角色 / 载体（数据而非代码）")
    p.add_argument("action", choices=["init", "list", "path"],
                   help="init=生成可编辑的角色文件；list=看当前生效角色；path=看用的哪个文件")
    p.add_argument("--json", action="store_true", help="init 写 JSON（默认 TOML，多行提示更友好）")
    p.add_argument("--force", action="store_true", help="init 覆盖已存在的文件")
    a = p.parse_args(argv)

    if a.action == "path":
        print(R.active_source())
        return 0
    if a.action == "list":
        try:
            stages = R.load_stages()
        except Exception as e:                # noqa: BLE001
            print("✗ 角色配置有误：", e)
            return 1
        print(f"来源：{R.active_source()}\n")
        for i, s in enumerate(stages, 1):
            mt = s.max_tokens or "—"
            print(f"  {i}. {s.key:12s} role={s.role:10s} carrier={s.carrier:18s} "
                  f"exec={s.executor:8s} max_tok={mt!s:6s} {s.title}")
        print(f"\n共 {len(stages)} 个角色。carrier 直接写后端名（deepseek/glm/...）则免改网关、免重启。")
        return 0
    # init
    ext = "json" if a.json else "toml"
    dest = pathlib.Path(f"loom.roles.{ext}")
    if dest.exists() and not a.force:
        print(f"✗ {dest} 已存在（加 --force 覆盖）")
        return 1
    stages = R.load_stages()                  # 以当前生效（默认或已有）为蓝本，从可跑基线改起
    dest.write_text(R.stages_to_json(stages) if a.json else R.stages_to_toml(stages),
                    encoding="utf-8")
    print(f"✓ 已生成 {dest}（{len(stages)} 个角色）。\n"
          f"  编辑它即可定义你自己的 Agent：每个 [[stages]] 改 carrier（指向后端/载体）与 system（角色契约）。\n"
          f"  carrier 直接写 deepseek/glm/minimax/codex-sub → 零额外配置、免重启。\n"
          f"  改完 `devkit run \"任务\"` 即生效，`devkit roles list` 可复核。")
    return 0


def _bar(pct, width=12):
    if pct is None:
        return " " * width
    n = int(round(min(max(pct, 0), 100) / 100 * width))
    return "█" * n + "·" * (width - n)


def _cmd_quota(argv) -> int:
    """额度薅羊毛：每个后端的已用 / 免费额度 / 剩余，并建议现在该薅哪个。"""
    import os
    from devkit import insight
    from devkit.rdloop import load_master_key
    if argv and argv[0] == "simulate":
        stages_str = argv[1] if len(argv) > 1 else ""
        stages = [s.strip() for s in stages_str.split(",") if s.strip()]
        if not stages:
            print("用法：devkit quota simulate <stage1,stage2,...>")
            return 1
        base_url = os.environ.get("LITELLM_BASE_URL", "http://localhost:4000")
        result = insight.quota_simulate(stages, base_url, load_master_key())
        print("阶段预估成本：")
        for s in stages:
            cost = result["stage_costs"].get(s)
            if cost is None:
                print(f"  {s}  $?（无历史）")
            else:
                print(f"  {s}  ${cost:.5f}（历史均值）")
        print(f"预估总计：${result['estimated_total']:.5f}")
        rem = result["remaining_usd"]
        print(f"可用额度：{'$?（未配置）' if rem is None else f'${rem:.5f}（免费）'}")
        v = result["verdict"]
        emoji = {"Safe": "✅", "Risky": "⚠️", "Insufficient": "❌"}.get(v, "❓")
        print(f"结论：{emoji} {v}")
        return 0
    p = argparse.ArgumentParser(prog="devkit quota",
                                description="模型额度薅羊毛看板（订阅/免费额度优先用）")
    p.add_argument("--base-url", default=os.environ.get("LITELLM_BASE_URL", "http://localhost:4000"))
    a = p.parse_args(argv)
    rep = insight.quota_report(a.base_url, load_master_key())
    if not rep["configured"]:
        print("提示：还没有 loom.quota.toml —— `cp loom.quota.example.toml loom.quota.toml` "
              "填上各后端的免费额度即可。下面按默认（全付费/订阅未声明）显示已用量。\n")
    if not rep["gateway_ok"]:
        print("⚠️ 网关不可达，已用量按 0 显示（仅展示你声明的额度）。\n")
    print(f"{'后端':<10}{'类型':<8}{'已用$':>10}{'免费$':>8}{'剩余$':>10}  进度")
    for r in rep["rows"]:
        free = f"{r['free_usd']:.2f}" if r["free_usd"] else "—"
        rem = f"{r['remaining_usd']:.4f}" if r["remaining_usd"] is not None else "—"
        print(f"{r['backend']:<10}{r['kind']:<8}{r['used_usd']:>10.5f}{free:>8}{rem:>10}  "
              f"{_bar(r['pct_used'])} {('' if r['pct_used'] is None else str(r['pct_used'])+'%')}")
    if rep["recommend"]:
        print(f"\n💡 现在优先薅：**{rep['recommend']}**（订阅或剩余免费额度最多）。")
    return 0


def _cmd_scores(argv) -> int:
    """模型评分：实际使用（成功率/延迟/成本）+ 你的 👍/👎 + 官网评测分 → 综合。"""
    from devkit import insight
    p = argparse.ArgumentParser(prog="devkit scores",
                                description="模型评分（实际使用 + 用户 + 官网，透明加权）")
    p.add_argument("--runs-dir")
    a = p.parse_args(argv)
    rep = (insight.score_report(pathlib.Path(a.runs_dir)) if a.runs_dir
           else insight.score_report())
    w = rep["weights"]
    print(f"权重：实际 {w['actual']} · 用户 {w['user']} · 官网 {w['official']}（按存在的项归一化）")
    if not rep["has_official"]:
        print("提示：官网分还没填 —— `cp loom.scores.example.toml loom.scores.toml` 自己按各家榜单填（Loom 不杜撰）。")
    print(f"\n{'后端':<10}{'次数':>5}{'成功率':>7}{'均延迟':>8}{'均$/次':>10}{'👍':>4}{'👎':>4}{'官网':>6}{'综合':>6}")
    for r in rep["rows"]:
        ok = f"{r['ok_rate']}%" if r["ok_rate"] is not None else "—"
        lat = f"{r['avg_lat']}s" if r["avg_lat"] is not None else "—"
        cost = f"{r['avg_cost']:.5f}" if r["avg_cost"] is not None else "—"
        off = str(int(r["official"])) if r["official"] is not None else "—"
        comp = str(r["composite"]) if r["composite"] is not None else "—"
        print(f"{r['backend']:<10}{r['uses']:>5}{ok:>7}{lat:>8}{cost:>10}"
              f"{r['up']:>4}{r['down']:>4}{off:>6}{comp:>6}")
    print("\n用 `devkit rate <后端> up|down` 记你的实际体验（计入综合）。")
    return 0


def _cmd_stages(argv) -> int:
    """按阶段透视：研发流程里 tokens/钱花在哪个阶段（brainstorm/plan/implement/…）。"""
    from devkit import insight
    p = argparse.ArgumentParser(prog="devkit stages",
                                description="按阶段聚合 run-log（uses/成功率/tokens/成本/占比）")
    p.add_argument("--runs-dir")
    a = p.parse_args(argv)
    rep = (insight.stage_report(pathlib.Path(a.runs_dir)) if a.runs_dir
           else insight.stage_report())
    rows = rep["rows"]
    if not rows:
        print("还没有跑过（devkit/runs 下没有 run-log.md）—— 先 `./loom run \"任务\"` 跑一遍。")
        return 0
    print(f"{'阶段':<12}{'次数':>5}{'成功率':>7}{'均tok':>8}{'均$/次':>10}"
          f"{'总tok':>9}{'总$':>11}{'占成本%':>9}")
    for r in rows:
        ok = f"{r['ok_rate']}%" if r["ok_rate"] is not None else "—"
        avgt = str(r["avg_tokens"]) if r["avg_tokens"] is not None else "—"
        avgc = f"{r['avg_cost']:.5f}" if r["avg_cost"] is not None else "—"
        print(f"{r['stage']:<12}{r['uses']:>5}{ok:>7}{avgt:>8}{avgc:>10}"
              f"{r['total_tokens']:>9}{r['total_cost']:>11.5f}{r['pct_cost']:>8}%")
    t = rep["totals"]
    tot_pct = "100.0" if t["cost"] else "0.0"
    print(f"{'合计':<12}{t['uses']:>5}{'':>7}{'':>8}{'':>10}"
          f"{t['tokens']:>9}{t['cost']:>11.5f}{tot_pct:>8}%")
    print("\n占比高的阶段优先换更便宜的载体 / 加上下文压缩（--compact-model）。")
    return 0


def _cmd_rate(argv) -> int:
    """记一次模型实际体验评分（👍/👎），计入 scores 综合分。"""
    from devkit import insight
    p = argparse.ArgumentParser(prog="devkit rate", description="给某个后端记一次 👍/👎")
    p.add_argument("backend", help="后端名：claude/codex/glm/deepseek/minimax（或任意模型名，自动归一）")
    p.add_argument("verdict", choices=["up", "good", "down", "bad"], help="up/good=👍，down/bad=👎")
    p.add_argument("--note", default="", help="可选备注")
    a = p.parse_args(argv)
    val = 1 if a.verdict in ("up", "good") else -1
    b = insight.add_rating(a.backend, val, a.note)
    print(f"✓ 已记：{b} {'👍' if val > 0 else '👎'}" + (f"（{a.note}）" if a.note else ""))
    return 0


def _cmd_backlog(argv) -> int:
    """Initializer：把应用想法拆成可增量交付的特性清单（或 --status 看进度）。"""
    import os
    from devkit import backlog as B
    from devkit.rdloop import load_master_key
    p = argparse.ArgumentParser(prog="devkit backlog", description="Initializer：拆特性清单")
    p.add_argument("idea", nargs="?", help="应用想法（初始化时必填）")
    p.add_argument("--dir", help="项目目录（默认 devkit/projects/<slug>）")
    p.add_argument("--features", type=int, default=8, help="拆成大约几个特性")
    p.add_argument("--carrier", default="loom-orchestrator")
    p.add_argument("--base-url", default=os.environ.get("LITELLM_BASE_URL", "http://localhost:4000"))
    p.add_argument("--status", action="store_true", help="只看进度")
    a = p.parse_args(argv)
    if a.status:
        pd = _locate_project(a.idea, a.dir)
        bl = B.load_backlog(pd)
        done, total = B.counts(bl)
        print(f"项目：{pd}\n进度：{done}/{total} 完成\n")
        for f in sorted(bl["features"], key=lambda x: x.get("priority", 99)):
            print(f"  {'✅' if f['status'] == 'done' else '⬜'} [{f['id']}] (p{f.get('priority')}) {f['title']}")
        return 0
    if not a.idea:
        p.error('初始化需要给应用想法，如：devkit backlog "一个命令行待办清单"')
    key = load_master_key()
    if not key:
        raise SystemExit("找不到 LITELLM_MASTER_KEY")
    r = B.init_backlog(a.idea, a.base_url, key, a.carrier, a.features,
                       pathlib.Path(a.dir) if a.dir else None)
    print(f"✓ Initializer：{r['features']} 个特性 → {r['project_dir']}  (+{r['tokens']}tok ${r['cost']:.5f})")
    print(f"  下一步：devkit feature --dir {r['project_dir']}（逐个增量构建）")
    return 0


def _cmd_feature(argv) -> int:
    """增量构建下一个（或连续 K 个）未完成特性，测试绿才标 done。"""
    import os
    from devkit import backlog as B
    from devkit.rdloop import load_master_key
    p = argparse.ArgumentParser(prog="devkit feature", description="增量构建下一个特性")
    p.add_argument("idea", nargs="?", help="用想法/slug 定位项目（或用 --dir）")
    p.add_argument("--dir", help="项目目录")
    p.add_argument("--count", type=int, default=1, help="连续构建几个特性")
    p.add_argument("--iterate", type=int, default=2, help="单特性测试失败时的迭代修复轮数")
    p.add_argument("--commit", action="store_true",
                   help="每个特性测试绿后就地 git 提交一次（首次自动 git init，不 push）")
    p.add_argument("--carrier", default="loom-dev")
    p.add_argument("--base-url", default=os.environ.get("LITELLM_BASE_URL", "http://localhost:4000"))
    a = p.parse_args(argv)
    pd = _locate_project(a.idea, a.dir)
    key = load_master_key()
    if not key:
        raise SystemExit("找不到 LITELLM_MASTER_KEY")
    for _ in range(max(a.count, 1)):
        r = B.build_feature(pd, a.base_url, key, a.carrier, a.iterate, commit=a.commit)
        if r.get("empty"):
            print(r["msg"])
            break
        f = r["feature"]
        if r.get("error"):
            print(f"✗ [{f['id']}] {f['title']}：{r['error']}")
            break
        if r["tests"] is False:
            print(f"✗ [{f['id']}] {f['title']} — {r['attempts']} 次未过，未标记 done（${r['cost']:.5f}）")
            break
        verdict = {True: "✅测试通过", None: "⚠️无测试"}[r["tests"]]
        cmsg = (f" · commit {r['commit']}" if r.get("commit")
                else (" · commit失败" if r.get("commit_error") else ""))
        print(f"✓ [{f['id']}] {f['title']} — {verdict} · {len(r['files'])}文件 · "
              f"{r['attempts']}次 · 进度 {r['done']}/{r['total']} · ${r['cost']:.5f}{cmsg}")
    return 0


def _color_diff(text: str) -> str:
    if not sys.stdout.isatty():
        return text
    out = []
    for ln in text.splitlines():
        if ln.startswith("+") and not ln.startswith("+++"):
            out.append("\033[32m" + ln + "\033[0m")
        elif ln.startswith("-") and not ln.startswith("---"):
            out.append("\033[31m" + ln + "\033[0m")
        elif ln.startswith("@@"):
            out.append("\033[36m" + ln + "\033[0m")
        else:
            out.append(ln)
    return "\n".join(out)


def _cmd_diff(argv) -> int:
    """对比两次运行的 build/ 产物（默认对比上一次带 build 的运行），与控制台共用 devkit.diff。"""
    from devkit.diff import diff_runs
    from devkit.rdloop import ROOT
    p = argparse.ArgumentParser(prog="devkit diff",
                                description="对比两次运行的 build/ 产物")
    p.add_argument("ts", help="本次运行 id/时间戳（runs/<ts>）")
    p.add_argument("--against", default="", help="对比基线运行 id（默认自动选上一次带 build 的）")
    p.add_argument("--runs-dir", default=str(ROOT / "devkit" / "runs"))
    a = p.parse_args(argv)
    res = diff_runs(pathlib.Path(a.runs_dir), a.ts, a.against)
    if "error" in res:
        print("✗", res["error"])
        return 1
    changed = [f for f in res["files"] if f["status"] != "same"]
    print(f"对比基线 {res['against']} · {res['changed']}/{res['total']} 文件有改动")
    if not changed:
        print("（无改动）")
        return 0
    badge = {"changed": "~", "new": "+", "deleted": "-"}
    for f in changed:
        print(f"\n{badge.get(f['status'], '?')} {f['name']}  [{f['status']}]")
        if f["diff"]:
            print(_color_diff(f["diff"]))
    return 0


def _cmd_radar(argv) -> int:
    """Community Radar：浏览/导入精选 MCP 服务器、技能规则到资产库。"""
    from devkit import radar as R
    p = argparse.ArgumentParser(prog="devkit radar",
                                description="社区雷达：浏览精选 MCP/规则/技能，扫描本地目录，导入到资产库")
    sub = p.add_subparsers(dest="action")

    lp = sub.add_parser("list", help="列出精选目录")
    lp.add_argument("--category", choices=["mcp", "rule", "skill"], help="过滤分类")

    sp = sub.add_parser("scan", help="扫描目录中的工具配置文件")
    sp.add_argument("path", nargs="?", default=".", help="要扫描的目录（默认当前目录）")

    ip = sub.add_parser("import", help="导入精选目录中的条目到资产库")
    ip.add_argument("name", help="条目名称（radar list 查看）")
    ip.add_argument("--assets-file", metavar="PATH", help="资产文件路径（默认自动查找）")

    ap = sub.add_parser("import-scan", help="扫描目录并导入所有找到的工具配置")
    ap.add_argument("path", nargs="?", default=".", help="要扫描的目录")
    ap.add_argument("--assets-file", metavar="PATH", help="资产文件路径")

    a = p.parse_args(argv)
    if not a.action:
        p.print_help()
        return 0

    if a.action == "list":
        entries = R.list_catalog(getattr(a, "category", None))
        if not entries:
            print("（无条目）")
            return 0
        print(f"{'名称':<28}{'分类':<8}{'信任':<6}  描述")
        print("-" * 80)
        for e in entries:
            from devkit import asset as A
            label = f"L{e.get('trust_level',0)}"
            print(f"  {e['name']:<26}{e.get('category','?'):<8}{label:<6}  {e.get('description','')[:40]}")
        print(f"\n共 {len(entries)} 条。用 `devkit radar import <name>` 导入。")
        return 0

    if a.action == "scan":
        results = R.scan_dir(pathlib.Path(a.path))
        if not results:
            print(f"在 {a.path} 中未找到已知工具配置文件。")
            return 0
        print(f"发现 {len(results)} 个配置文件：")
        for r in results:
            print(f"  {r['name']:<24}{r['category']:<8}  {r['source']}")
        print(f"\n用 `devkit radar import-scan {a.path}` 全部导入到资产库。")
        return 0

    if a.action == "import":
        assets_path = pathlib.Path(a.assets_file) if getattr(a, "assets_file", None) else None
        result = R.import_to_assets(a.name, assets_path=assets_path)
        if result is None:
            print(f"找不到条目 {a.name!r}（用 devkit radar list 查看可用条目）")
            return 1
        from devkit import asset as A
        print(f"✓ 导入：{result['name']}（{result['type']}，trust=L{result['trust_level']} {A.trust_label(result['trust_level'])}）")
        return 0

    if a.action == "import-scan":
        assets_path = pathlib.Path(a.assets_file) if getattr(a, "assets_file", None) else None
        results = R.scan_dir(pathlib.Path(a.path))
        if not results:
            print(f"在 {a.path} 中未找到已知工具配置文件。")
            return 0
        from devkit import asset as A
        imported = 0
        for r in results:
            created = R.import_to_assets(r["name"], assets_path=assets_path, scan_result=r)
            if created:
                print(f"  ✓ {created['name']}（trust=L{created['trust_level']}，待审查 → devkit asset trust {created['name']} 1）")
                imported += 1
        print(f"\n导入 {imported} 个资产。所有新资产 trust_level=0，建议逐一审查后提升信任等级。")
        return 0

    return 0


def _cmd_migrate(argv) -> int:
    """Workflow Migration：把其他 AI 工具的配置导入 Loom 资产库。"""
    from devkit import migrate as M
    p = argparse.ArgumentParser(prog="devkit migrate",
                                description="迁移其他 AI 工具配置（claude-code/aider/cline/roo/cursor）到资产库")
    sub = p.add_subparsers(dest="action")

    dp = sub.add_parser("detect", help="检测当前目录中有哪些 AI 工具配置")
    dp.add_argument("--dir", default=".", metavar="PATH", help="要检测的目录（默认当前）")

    tp = sub.add_parser("tool", help="导入指定工具的配置")
    tp.add_argument("tool_name",
                    choices=["claude-code", "aider", "cline", "roo", "cursor",
                             "codex", "continue", "openclaw", "hermes"],
                    help="工具名称")
    tp.add_argument("--dir", default=".", metavar="PATH", help="工具配置所在目录")
    tp.add_argument("--assets-file", metavar="PATH", help="资产文件路径")

    allp = sub.add_parser("all", help="自动检测并导入所有找到的工具配置")
    allp.add_argument("--dir", default=".", metavar="PATH", help="要扫描的目录")
    allp.add_argument("--assets-file", metavar="PATH", help="资产文件路径")

    a = p.parse_args(argv)
    if not a.action:
        p.print_help()
        return 0

    if a.action == "detect":
        detected = M.detect(pathlib.Path(a.dir))
        found = {k: v for k, v in detected.items() if v["found"]}
        not_found = {k: v for k, v in detected.items() if not v["found"]}
        if found:
            print(f"✅ 发现 {len(found)} 个工具：")
            for tool, info in found.items():
                print(f"  {tool:<16}  {info['description']}  →  {', '.join(info['files'])}")
            print(f"\n用 `devkit migrate all` 一键导入，或 `devkit migrate tool <name>` 单独导入。")
        else:
            print("未发现已知 AI 工具配置文件。")
        if not_found:
            print(f"\n未检测到：{', '.join(not_found)}")
        return 0

    if a.action == "tool":
        assets_path = pathlib.Path(a.assets_file) if getattr(a, "assets_file", None) else None
        created = M.migrate_tool(a.tool_name, pathlib.Path(a.dir), assets_path)
        if not created:
            print(f"在 {a.dir} 中未找到 {a.tool_name} 的配置文件。")
            return 1
        from devkit import asset as A
        for asset in created:
            print(f"✓ {asset['name']}（{asset['type']}，trust=L{asset['trust_level']} {A.trust_label(asset['trust_level'])}）")
        print(f"\n导入 {len(created)} 个资产。用 `devkit asset trust <name> <level>` 提升信任等级。")
        return 0

    if a.action == "all":
        assets_path = pathlib.Path(a.assets_file) if getattr(a, "assets_file", None) else None
        results = M.migrate_all(pathlib.Path(a.dir), assets_path)
        if not results:
            print("未发现任何可迁移的工具配置。")
            return 0
        from devkit import asset as A
        total = 0
        for tool_name, assets in results.items():
            for asset in assets:
                print(f"  [{tool_name}] ✓ {asset['name']}（trust=L{asset['trust_level']}）")
                total += 1
        print(f"\n共导入 {total} 个资产，来自 {len(results)} 个工具。")
        print("建议：逐一 `devkit asset trust <name> <level>` 审查后提升信任等级。")
        return 0

    return 0


def _cmd_init(argv) -> int:
    """在当前目录创建 loom.toml 项目配置文件。"""
    p = argparse.ArgumentParser(prog="devkit init",
                                description="在当前目录创建 loom.toml 项目默认配置")
    p.add_argument("--force", action="store_true", help="覆盖已有的 loom.toml")
    a = p.parse_args(argv)
    from devkit import config as _cfg
    dest = pathlib.Path.cwd() / "loom.toml"
    if dest.exists() and not a.force:
        print(f"loom.toml 已存在（{dest}）。用 --force 覆盖。")
        return 1
    _cfg.write_default_config(dest)
    print(f"✓ 创建 {dest}")
    print("  编辑文件设置项目默认值，devkit run 会自动读取。")
    return 0


def _cmd_config(argv) -> int:
    """显示当前生效的 loom.toml 配置。"""
    p = argparse.ArgumentParser(prog="devkit config",
                                description="显示当前生效的 loom.toml 配置")
    p.add_argument("--path", action="store_true", help="只显示配置文件路径")
    a = p.parse_args(argv)
    from devkit import config as _cfg
    cfg_path = _cfg.find_config()
    if a.path:
        print(str(cfg_path) if cfg_path else "（未找到 loom.toml）")
        return 0
    if cfg_path is None:
        print("未找到 loom.toml（从当前目录向上找），用 `devkit init` 创建。")
        return 0
    print(f"配置文件：{cfg_path}\n")
    conf = _cfg.load_config(cfg_path)
    if not conf:
        print("（文件为空或全部注释）")
        return 0
    for k, v in conf.items():
        if k == "_carrier":
            for entry in v:
                print(f"  carrier.{entry}")
        else:
            print(f"  {k} = {v!r}")
    return 0


def _cmd_learn(argv) -> int:
    """Learning Sidecar：读历史 run，输出 carrier/quota/safety 建议。"""
    from devkit.rdloop import ROOT
    p = argparse.ArgumentParser(prog="devkit learn",
                                description="分析历史 run，输出 carrier/quota/safety 建议（只读）")
    p.add_argument("--runs-dir", default=str(ROOT / "devkit" / "runs"),
                   help="runs 目录（默认 devkit/runs）")
    p.add_argument("--task-type", metavar="TYPE",
                   help="只分析特定任务类型的 carrier 建议")
    p.add_argument("--json", action="store_true", dest="as_json",
                   help="以 JSON 输出（适合管道）")
    p.add_argument("--goldens", action="store_true",
                   help="额外分析 golden 测试失败模式，给出修复建议")
    p.add_argument("--discover", action="store_true",
                   help="自动发现候选任务并按价值评分（调用 discover + valuer）")
    a = p.parse_args(argv)

    from devkit import learn as _learn
    runs_dir = pathlib.Path(a.runs_dir)

    if a.task_type:
        sug = _learn.suggest_carrier(a.task_type, runs_dir)
        if a.as_json:
            import json as _json
            print(_json.dumps(sug, ensure_ascii=False, indent=2))
        elif sug:
            print(f"[{sug['type']}] confidence={sug['confidence']}")
            print(f"  原因：{sug['reason']}")
            print(f"  建议：{sug['action']}")
        else:
            print(f"暂无针对 {a.task_type!r} 的 carrier 建议（历史数据不足）")
        return 0

    result = _learn.analyze(runs_dir)
    trend = _learn.quota_trend(runs_dir)
    golden_sugs = _learn.suggest_goldens(runs_dir) if a.goldens else []

    if a.as_json:
        import json as _json
        print(_json.dumps({"analyze": result, "quota_trend": trend,
                           "golden_suggestions": golden_sugs},
                          ensure_ascii=False, indent=2))
        return 0

    s = result["summary"]
    print(f"Learning Sidecar  ·  历史 {s['total_runs']} 次 run  "
          f"GO率 {s['go_rate']:.0%}  均成本 ${s['avg_cost_usd']:.5f}"
          f"  总计 ${s['total_cost_usd']:.4f}")
    print(f"成本趋势：{trend['trend']}  (近10次均 ${trend['avg_cost']:.5f}，"
          f"峰值 ${trend['max_cost']:.5f})")

    sugs = result["suggestions"] + golden_sugs
    if not sugs:
        print("\n暂无建议（历史数据不足，多跑几次 run 后再看）")
    else:
        print(f"\n{len(sugs)} 条建议：")
        badges = {"carrier": "🔄", "quota": "💸", "safety": "🚨", "golden": "🧪"}
        for i, sug in enumerate(sugs, 1):
            badge = badges.get(sug["type"], "?")
            print(f"\n  {i}. {badge} [{sug['type']}]  confidence={sug['confidence']:.0%}")
            print(f"     {sug['reason']}")
            print(f"     → {sug['action']}")

    if a.discover:
        from devkit import discover as _discover, valuer as _valuer, learn as _learn2
        fitness_rows = _learn2._load_fitness(runs_dir)
        # map learn suggestions (confidence float) to discover expected priority field
        def _conf_to_priority(c):
            return "high" if c >= 0.6 else ("medium" if c >= 0.3 else "low")
        discover_sugs = [dict(s, priority=_conf_to_priority(s.get("confidence", 0))) for s in sugs]
        cands_fitness = _discover.from_fitness(fitness_rows)
        cands_sugs = _discover.from_suggestions(discover_sugs)
        candidates = _discover.merge([cands_fitness, cands_sugs], max_total=10)
        if not candidates:
            print("\n🔍 自动发现：暂无候选任务（历史数据不足）")
        else:
            scored = _valuer.top_n(candidates, [{} for _ in candidates], n=5)
            print(f"\n🔍 自动发现候选任务（top {len(scored)}）：")
            for j, r in enumerate(scored, 1):
                c = r["candidate"]
                print(f"  {j}. [{r['score']}分] {c.get('type','?')} "
                      f"task_type={c.get('task_type','?')} backend={c.get('backend','?')}")
                print(f"     {r['reason']}")
    return 0


def _refill_backlog(bl_path, backlog: list, max_new: int = 5) -> int:
    """backlog 清空时自动 discover+valuer 补充新任务，返回新增数量。"""
    import json as _json
    from devkit import discover as _discover, valuer as _valuer
    try:
        from devkit.rdloop import run_loop, ROOT, load_master_key
        import os as _os
        base_url = _os.environ.get("LITELLM_BASE_URL", "http://localhost:4000")
        api_key = load_master_key()
        suggestions = []
        if api_key:
            from devkit import insight as _insight
            fitness = _insight.model_fitness().get("rows", [])
            suggestions = _discover.from_fitness(fitness)
    except Exception:
        suggestions = []

    candidates = _discover.merge([suggestions], max_total=max_new * 2)
    if not candidates:
        return 0

    scored = _valuer.top_n(
        candidates,
        [{"priority": c.get("priority", "medium")} for c in candidates],
        n=max_new,
    )
    done_ids = {t["id"] for t in backlog}
    added = 0
    for r in scored:
        c = r["candidate"]
        tid = c.get("type", "auto") + "-" + str(len(backlog) + added + 1)
        if tid in done_ids:
            continue
        backlog.append({
            "id": tid,
            "task": c.get("description", c.get("type", "auto-discovered task")),
            "status": "pending",
            "priority": "high" if r["score"] >= 70 else "medium",
            "deps": [],
            "carrier": {"implement": "minimax"},
        })
        added += 1
        if added >= max_new:
            break

    if added:
        merged = _write_backlog(bl_path, backlog)
        backlog[:] = merged
    return added


def _record_backlog_decision(item: dict, run_args: dict, backlog: list) -> None:
    from devkit import decision_log as _dlog, valuer as _valuer
    pending = [e for e in backlog if e.get("status") == "pending"]
    scored = _valuer.top_n(
        pending,
        [{"priority": e.get("priority", "medium")} for e in pending],
        n=min(5, len(pending)),
    ) if pending else []
    chosen_scored = next(
        (r for r in scored if r["candidate"].get("id") == item.get("id")), {}
    )
    alts = [
        {"task_id": r["candidate"].get("id"), "score": r["score"], "reason": r["reason"]}
        for r in scored if r["candidate"].get("id") != item.get("id")
    ]
    _dlog.append(
        task_id=item.get("id", "?"),
        task_text=run_args["task"],
        run_id=run_args["run_id"],
        score=chosen_scored.get("score", 0),
        reason=chosen_scored.get("reason", ""),
        alternatives=alts,
        sync_backlog=True,
        priority=item.get("priority"),
    )


def _run_selected_backlog_item(backlog: list, bl_path: pathlib.Path, item: dict, run_args: dict):
    from devkit import autoloop as _autoloop, decision_log as _dlog, lease as _lease
    from devkit.rdloop import run_loop
    from devkit.roles import load_stages
    import json as _json

    for entry in backlog:
        if entry.get("id") == item.get("id"):
            entry["status"] = _autoloop.advance_state(entry.get("status", "pending"), "start")
            _lease.attach_lease(entry, owner_pid=os.getpid(), run_id=run_args["run_id"])
    merged = _write_backlog(bl_path, backlog)
    backlog[:] = merged

    carrier_map = {}
    for pair in run_args["carriers"]:
        if "=" in pair:
            k, v = pair.split("=", 1)
            v = v.strip()
            if v.lower().endswith("/m3"):
                v = v.split("/")[0]
            k = k.strip()
            carrier_map[k] = normalize_model_name(v, stage=k)
    executor_map = {}
    for pair in run_args.get("executors", []):
        if "=" in pair:
            k, v = pair.split("=", 1)
            executor_map[k.strip()] = v.strip()
    stages_arg = [s.strip() for s in run_args["stages"].split(",") if s.strip()]
    all_stages = load_stages()
    by_key = {s.key: s for s in all_stages}
    selected = [by_key[k] for k in stages_arg if k in by_key]
    from devkit.delivery_mode import resolve_delivery_mode, resolved_targets

    delivery_mode = resolve_delivery_mode(
        delivery_mode=run_args.get("delivery_mode"),
        apply_target=run_args.get("apply_target"),
        apply_git=run_args.get("apply_git"),
    )
    apply_target = run_args.get("apply_target")
    apply_git = run_args.get("apply_git")
    apply_branch = run_args.get("apply_branch")
    repo_root = pathlib.Path(__file__).resolve().parent.parent
    apply_target, apply_git = resolved_targets(
        mode=delivery_mode,
        repo_root=repo_root,
        apply_target=apply_target,
        apply_git=apply_git,
    )

    result = run_loop(
        task=run_args["task"],
        stages=selected or None,
        carrier_overrides=carrier_map,
        executor_map=executor_map,
        task_kind=run_args.get("task_kind"),
        allowed_artifact_paths=run_args.get("allowed_artifact_paths"),
        forbidden_artifact_paths=run_args.get("forbidden_artifact_paths"),
        delivery_mode=delivery_mode,
        apply_target=apply_target,
        apply_git=apply_git,
        apply_branch=apply_branch,
        run_id=run_args["run_id"],
        iterate=int(run_args.get("iterate", 0) or 0),
        contract=int(run_args.get("contract", 0) or 0),
        contract_rounds=int(run_args.get("contract_rounds", 0) or 0),
        budget=run_args.get("budget"),
        blind_review=bool(run_args.get("blind_review", False)),
        physical_verify=bool(run_args.get("physical_verify", False)),
        compact_model=run_args.get("compact_model", "deepseek"),
        cascade=([c.strip() for c in run_args["cascade"].split(",") if c.strip()]
                 if isinstance(run_args.get("cascade"), str) else run_args.get("cascade")),
        health_probe=bool(run_args.get("health_probe", False)),
    )
    event = "success" if _autoloop.is_success_gate(result.get("gate")) else "failure"
    for entry in backlog:
        if entry.get("id") == item.get("id"):
            entry["status"] = _autoloop.advance_state("running", event)
    merged = _write_backlog(bl_path, backlog)
    backlog[:] = merged
    _dlog.update_outcome(run_args["run_id"], event, sync_backlog=True, backlog_path=bl_path)
    print(f"✓ 任务 {item.get('id')} → {event}（backlog 已更新）")
    return event, result


def _load_run_verdict(run_dir):
    """Load the Phase D GateVerdict payload from ``<run_dir>/verdict.json``.

    Phase F: ``run_loop`` is expected to write a typed
    :class:`devkit.gatekeeper.GateVerdict` to ``verdict.json`` after
    Phase E's ``unify-run-gate`` lands. The reflection loop consumes
    that verdict via :func:`devkit.iterate.infer_failure_code` so the
    repairer gets a code it can act on, instead of the legacy text-regex
    guess.

    This helper is fail-open: any I/O / parse error returns ``None``
    so the caller falls back to the existing text-regex path. The
    verdict payload is returned in its wire shape (a plain ``dict``) so
    ``infer_failure_code`` can introspect ``spec.passed`` and
    ``spec.failure_codes`` directly.
    """
    if not run_dir:
        return None
    try:
        import json as _json
        import pathlib as _pathlib
        verdict_path = _pathlib.Path(run_dir) / "verdict.json"
        if not verdict_path.exists():
            return None
        with verdict_path.open("r", encoding="utf-8") as fh:
            return _json.load(fh)
    except Exception:  # noqa: BLE001 — fail-open: never block reflection
        return None


def _cmd_auto(argv) -> int:
    """Layer B 自治驱动循环：从 backlog.json 中自动选取就绪任务并依次运行。"""
    from devkit import autoloop as _autoloop, decision_log as _dlog
    from devkit.rdloop import ROOT
    import json as _json
    p = argparse.ArgumentParser(prog="devkit auto",
                                description="自治驱动循环：自动选取 backlog 中就绪任务并运行")
    p.add_argument("vision", nargs="?", default=None,
                   help="愿景字符串：从自然语言生成 backlog 并运行（Layer B 产品模式）")
    p.add_argument("--backlog", default=str(ROOT / "devkit" / "backlog.json"),
                   help="任务清单 JSON 路径（默认 devkit/backlog.json）")
    p.add_argument("--dry-run", action="store_true",
                   help="只打印下一个就绪任务，不真正运行")
    p.add_argument("--json", action="store_true", dest="as_json",
                   help="以 JSON 输出 run_once 参数（配合脚本使用）")
    p.add_argument("--yes", "-y", action="store_true",
                   help="跳过人类优先级确认（T12 gate），直接运行")
    p.add_argument("--loop", action="store_true",
                   help="自动循环：持续运行 backlog 中所有就绪任务直到清空")
    a = p.parse_args(argv)

    # Layer B 产品模式：从愿景生成 backlog（T13）
    if a.vision:
        return _auto_from_vision(a)

    bl_path = pathlib.Path(a.backlog)
    if not bl_path.exists():
        print(f"找不到 backlog 文件：{bl_path}（用 `devkit backlog` 初始化，或传入愿景字符串）")
        return 1
    try:
        backlog = _json.loads(bl_path.read_text(encoding="utf-8"))
    except Exception as e:  # noqa: BLE001
        print(f"backlog.json 解析失败：{e}")
        return 1
    _dlog.reconcile_pending_with_backlog(backlog_path=bl_path)
    backlog = _reclaim_stale_running(bl_path, backlog)
    backlog = _prune_stale_pending(bl_path, backlog)
    backlog = _prune_human_only_pending(bl_path, backlog)

    total_run = 0
    while True:
        backlog = _reclaim_stale_running(bl_path, backlog)
        backlog = _prune_stale_pending(bl_path, backlog)
        backlog = _prune_human_only_pending(bl_path, backlog)
        item = _autoloop.pick_next(backlog)
        if item is None:
            if a.loop:
                added = _refill_backlog(bl_path, backlog)
                if added:
                    print(f"♻ backlog 已补充 {added} 个新任务，继续循环…")
                    backlog = _json.loads(bl_path.read_text(encoding="utf-8"))
                    continue
            if total_run == 0:
                print("✅ backlog 中无就绪任务（所有任务已完成或存在未满足的依赖）")
            else:
                print(f"✅ backlog 全部完成（共运行 {total_run} 个任务）")
            return 0

        run_args = _autoloop.run_once(item)

        if a.as_json:
            print(_json.dumps(run_args, ensure_ascii=False, indent=2))
            return 0

        if a.dry_run:
            print(f"（dry-run）选中任务：{item.get('id','?')}")
            print(f"  task   = {run_args['task']!r}")
            print(f"  stages = {run_args['stages']}")
            print(f"  run_id = {run_args['run_id']}")
            return 0

        # 对所有就绪任务打分（无论 --yes 与否，决策日志都需要）
        from devkit import valuer as _valuer, decision_log as _dlog
        _pending = [e for e in backlog if e.get("status") == "pending"]
        _scored = _valuer.top_n(
            _pending,
            [{"priority": e.get("priority", "medium")} for e in _pending],
            n=min(5, len(_pending)),
        ) if _pending else []
        _chosen_scored = next(
            (r for r in _scored if r["candidate"].get("id") == item.get("id")), {}
        )
        _alts = [
            {"task_id": r["candidate"].get("id"), "score": r["score"], "reason": r["reason"]}
            for r in _scored if r["candidate"].get("id") != item.get("id")
        ]
        # 落盘决策记录（outcome=pending，运行后更新）
        _dlog.append(
            task_id=item.get("id", "?"),
            task_text=run_args["task"],
            run_id=run_args["run_id"],
            score=_chosen_scored.get("score", 0),
            reason=_chosen_scored.get("reason", ""),
            alternatives=_alts,
            sync_backlog=True,
            backlog_path=bl_path,
            priority=item.get("priority"),
        )

        # T12 人类优先级门：展示候选任务评分，等待确认
        if not a.yes:
            if len(_pending) > 1:
                print(f"\n📋 就绪任务（按价值评分，共 {len(_pending)} 个）：")
                for _j, _r in enumerate(_scored, 1):
                    _c = _r["candidate"]
                    _marker = "◀ 自动选中" if _c.get("id") == item.get("id") else ""
                    print(f"  {_j}. [{_r['score']}分] {_c.get('id','?')} — {_c.get('task','')[:60]} {_marker}")
            print(f"\n▶ 准备运行：{item.get('id','?')}")
            print(f"  task  = {run_args['task']!r}")
            print(f"  stages = {run_args['stages']}")
            print(f"  run_id = {run_args['run_id']}")
            try:
                ans = input("\n确认运行此任务？[y/N/跳过=s] ").strip().lower()
            except (EOFError, KeyboardInterrupt):
                print("\n已取消")
                return 1
            if ans == "s":
                for entry in backlog:
                    if entry.get("id") == item.get("id"):
                        entry["status"] = "skipped"
                merged = _write_backlog(bl_path, backlog)
                backlog[:] = merged
                _dlog.update_outcome(run_args["run_id"], "skipped", sync_backlog=True, backlog_path=bl_path)
                print(f"  跳过 {item.get('id')}，继续下一个…")
                if not a.loop:
                    return 0
                continue
            if ans not in ("y", "yes", "是", "ok"):
                _dlog.update_outcome(run_args["run_id"], "cancelled", sync_backlog=True, backlog_path=bl_path)
                print("已取消")
                return 1

        event, _result = _run_selected_backlog_item(backlog, bl_path, item, run_args)
        total_run += 1

        if not a.loop:
            return 0 if event == "success" else 1
        # loop 模式：失败不停，继续下一个（失败的留在 failed 状态）
        backlog = _json.loads(bl_path.read_text(encoding="utf-8"))


def _cmd_iterate(argv) -> int:
    """Run one task per round, then reflect and optionally rewrite backlog before continuing."""
    from devkit import autoloop as _autoloop, decision_log as _dlog, iterate as _iterate
    from devkit.ask import ask_one_with_fallback
    from devkit.rdloop import ROOT, load_master_key
    import json as _json, os as _os

    p = argparse.ArgumentParser(
        prog="devkit iterate",
        description="自动迭代：每轮执行一个任务，反思后写回 backlog，再继续下一轮",
    )
    p.add_argument("--backlog", default=str(ROOT / "devkit" / "backlog.json"))
    p.add_argument("--max-rounds", type=int, default=20, help="最多跑多少轮（默认 20）")
    p.add_argument("--reflect-carrier", default="minimax",
                   help="反思阶段使用的模型/载体，支持逗号分隔备选列表（默认 minimax）")
    p.add_argument("--compact-model", default="deepseek", metavar="MODEL",
                   help="迭代任务上下文压缩模型（默认 deepseek）")
    p.add_argument("--no-compact", action="store_true",
                   help="关闭上下文压缩，回到截断模式")
    p.add_argument("--reflect-max-tokens", type=int, default=1400)
    p.add_argument("--dry-run", action="store_true", help="只看下一轮会选什么")
    a = p.parse_args(argv)

    bl_path = pathlib.Path(a.backlog)
    if not bl_path.exists():
        print(f"找不到 backlog 文件：{bl_path}")
        return 1

    try:
        backlog = _json.loads(bl_path.read_text(encoding="utf-8"))
    except Exception as e:  # noqa: BLE001
        print(f"backlog.json 解析失败：{e}")
        return 1
    _dlog.reconcile_pending_with_backlog(backlog_path=bl_path)
    backlog = _reclaim_stale_running(bl_path, backlog)
    backlog = _prune_stale_pending(bl_path, backlog)
    backlog = _prune_human_only_pending(bl_path, backlog)

    total_run = 0
    reflections_dir = ROOT / "devkit" / "reflections"
    reflections_dir.mkdir(parents=True, exist_ok=True)
    base_url = _os.environ.get("LITELLM_BASE_URL", "http://localhost:4000")
    api_key = load_master_key()
    last_event = "success"

    for round_no in range(1, a.max_rounds + 1):
        backlog = _json.loads(bl_path.read_text(encoding="utf-8"))
        backlog = _reclaim_stale_running(bl_path, backlog)
        backlog = _prune_stale_pending(bl_path, backlog)
        backlog = _prune_human_only_pending(bl_path, backlog)
        item = _autoloop.pick_next(backlog)
        reflect_models = [m.strip() for m in a.reflect_carrier.split(",") if m.strip()]
        if not reflect_models:
            reflect_models = ["minimax"]
        if item is None:
            recent = _dlog.load(last_n=5)
            stalled_prompt = _iterate.build_reflection_prompt(
                round_no=round_no,
                task_id="backlog-recovery",
                outcome="stalled",
                gate="STALLED_NO_READY_TASK",
                backlog=backlog,
                run_log="No ready task was available. Reflect on failed or blocked dependencies and decide whether to requeue or add one concrete repair task.",
                recent_decisions=recent,
            )
            stalled_reflection = {
                "summary": "",
                "continue": True,
                "stop_reason": "",
                "requeue": [],
                "reprioritize": [],
                "add_tasks": [],
                "_raw": "",
            }
            if api_key and any(item.get("status") in {"failed", "pending", "stopped"} for item in backlog):
                print("↺ 无就绪任务，启动 stalled reflection")
                resp = ask_one_with_fallback(
                    reflect_models,
                    stalled_prompt,
                    base_url,
                    api_key,
                    max_tokens=a.reflect_max_tokens,
                    tag="iterate-stalled",
                )
                if resp.get("ok"):
                    stalled_reflection = _iterate.parse_reflection(resp.get("content", ""))
                else:
                    stalled_reflection["_parse_error"] = "reflect_call_failed"
                    stalled_reflection["_raw"] = resp.get("error", "")
                    stalled_reflection["summary"] = f"stalled reflect failed: {resp.get('error', '')[:160]}"
                stalled_applied = _iterate.apply_reflection(backlog, stalled_reflection)
                if (stalled_applied["changes"]["requeued"]
                        or stalled_applied["changes"]["reprioritized"]
                        or stalled_applied["changes"]["added"]):
                    backlog_after = _write_backlog(bl_path, stalled_applied["backlog"])
                    (reflections_dir / f"{round_no:02d}-stalled.md").write_text(
                        _iterate.reflection_markdown(
                            round_no=round_no,
                            task_id="backlog-recovery",
                            run_id="stalled",
                            outcome="stalled",
                            gate="STALLED_NO_READY_TASK",
                            reflection=stalled_applied.get("reflection", stalled_reflection),
                            changes=stalled_applied["changes"],
                        ),
                        encoding="utf-8",
                    )
                    if _autoloop.pick_next(backlog_after) is not None:
                        print("↺ stalled reflection 已补出就绪任务，继续下一轮")
                        continue
            if total_run == 0:
                print("✅ backlog 中无就绪任务（所有任务已完成或存在未满足的依赖）")
            else:
                print(f"✅ 自动迭代结束：共运行 {total_run} 轮，当前无就绪任务")
            return 0 if last_event == "success" else 1

        run_args = _autoloop.run_once(item)
        if a.dry_run:
            print(f"（dry-run）第 {round_no} 轮将选中任务：{item.get('id','?')}")
            print(f"  task   = {run_args['task']!r}")
            print(f"  stages = {run_args['stages']}")
            print(f"  run_id = {run_args['run_id']}")
            return 0

        if a.no_compact:
            run_args["compact_model"] = None
        else:
            run_args["compact_model"] = a.compact_model

        _record_backlog_decision(item, run_args, backlog)
        event, result = _run_selected_backlog_item(backlog, bl_path, item, run_args)
        total_run += 1
        last_event = event

        backlog_after = _json.loads(bl_path.read_text(encoding="utf-8"))
        run_dir = pathlib.Path(result.get("run_dir", ""))
        run_log = (run_dir / "run-log.md").read_text(encoding="utf-8") if run_dir.exists() else ""
        recent = _dlog.load(last_n=50)
        prompt = _iterate.build_reflection_prompt(
            round_no=round_no,
            task_id=item.get("id", "?"),
            outcome=event,
            gate=result.get("gate", ""),
            backlog=backlog_after,
            run_log=run_log,
            recent_decisions=recent,
        )

        reflection = {
            "summary": "",
            "continue": True,
            "stop_reason": "",
            "requeue": [],
            "reprioritize": [],
            "add_tasks": [],
            "_raw": "",
        }
        if api_key:
            resp = ask_one_with_fallback(
                reflect_models,
                prompt,
                base_url,
                api_key,
                max_tokens=a.reflect_max_tokens,
                tag="iterate-reflect",
            )
            if resp.get("ok"):
                reflection = _iterate.parse_reflection(resp.get("content", ""))
            else:
                reflection["_parse_error"] = "reflect_call_failed"
                reflection["_raw"] = resp.get("error", "")
                reflection["summary"] = f"reflect call failed: {resp.get('error', '')[:160]}"
        else:
            reflection["_parse_error"] = "missing_master_key"
            reflection["summary"] = "reflect skipped: missing LITELLM_MASTER_KEY"

        failure_code = _iterate.infer_failure_code(
            reflection.get("stop_reason", ""),
            reflection.get("summary", ""),
            result.get("gate", ""),
            run_log,
            gate_verdict=_load_run_verdict(run_dir),
        )
        applied = _iterate.apply_reflection(
            backlog_after,
            reflection,
            current_task_id=item.get("id", "?"),
            current_outcome=event,
            current_failure_code=failure_code,
            recent_records=recent,
        )
        reflection = applied.get("reflection", reflection)
        if applied["changes"]["requeued"] or applied["changes"]["reprioritized"] or applied["changes"]["added"]:
            backlog_after = _write_backlog(bl_path, applied["backlog"])
        _dlog.update_outcome(
            run_args["run_id"],
            event,
            sync_backlog=False,
            backlog_path=bl_path,
            reason=reflection.get("summary", "").strip() or result.get("gate", ""),
            root_cause=failure_code,
            next_action=_iterate.next_action_text(reflection),
            failure_code=failure_code,
        )
        reflection_md = _iterate.reflection_markdown(
            round_no=round_no,
            task_id=item.get("id", "?"),
            run_id=run_args["run_id"],
            outcome=event,
            gate=result.get("gate", ""),
            reflection=reflection,
            changes=applied["changes"],
        )
        (reflections_dir / f"{round_no:02d}-{run_args['run_id']}.md").write_text(
            reflection_md,
            encoding="utf-8",
        )

        ready_after = _autoloop.pick_next(backlog_after)
        print(
            f"↺ 反思 {round_no}: continue={reflection.get('continue', True)} "
            f"requeued={applied['changes']['requeued']} "
            f"reprioritized={applied['changes']['reprioritized']} "
            f"added={applied['changes']['added']}"
        )
        if not reflection.get("continue", True) and ready_after is None:
            reason = reflection.get("stop_reason", "").strip() or "reflection requested stop"
            print(f"🛑 反思停止：{reason}")
            return 0 if event == "success" else 1
        if not reflection.get("continue", True) and ready_after is not None:
            print("↺ 反思要求停止，但已补出新的就绪任务，继续自动迭代")
        if ready_after is None:
            print("✅ 反思后无就绪任务，停止自动迭代")
            return 0 if last_event == "success" else 1

    print(f"⏸ 已达到 max-rounds={a.max_rounds}，停止自动迭代")
    return 0 if last_event == "success" else 1


def _cmd_bench(argv) -> int:
    """对所有 carriers 跑标准 benchmark，打印对比表。"""
    import json as _json, os as _os
    p = argparse.ArgumentParser(prog="devkit bench", description="Carrier benchmark 测试")
    p.add_argument("--carriers", default="minimax,glm,deepseek",
                   help="逗号分隔的 carrier 列表（默认 minimax,glm,deepseek）")
    p.add_argument("--timeout", type=int, default=60, help="单请求超时秒数（默认 60）")
    p.add_argument("--save", action="store_true", help="结果写入 devkit/carrier_bench.json")
    p.add_argument("--json", action="store_true", dest="as_json", help="JSON 输出")
    a = p.parse_args(argv)

    from devkit import carrier_bench as _bench
    from devkit.rdloop import ROOT, load_master_key
    base_url = _os.environ.get("LITELLM_BASE_URL", "http://localhost:4000")
    api_key = load_master_key()
    if not api_key:
        raise SystemExit("找不到 LITELLM_MASTER_KEY")

    carriers = [c.strip() for c in a.carriers.split(",") if c.strip()]
    print(f"▶ Carrier Benchmark  carriers={carriers}  timeout={a.timeout}s\n")
    results = _bench.run_bench(carriers, base_url, api_key, timeout=a.timeout)

    if a.as_json:
        print(_json.dumps(results, ensure_ascii=False, indent=2))
    else:
        _bench.print_table(results)

    if a.save:
        _bench.save_results(results)
        print(f"\n结果已写入 {ROOT}/devkit/carrier_bench.json")
    return 0


def _cmd_decisions(argv) -> int:
    """查看自治决策日志（decisions.jsonl）。"""
    import json as _json
    p = argparse.ArgumentParser(prog="devkit decisions", description="查看自治决策日志")
    p.add_argument("-n", "--last", type=int, default=20, metavar="N",
                   help="显示最近 N 条决策（默认 20，0=全量）")
    p.add_argument("--json", action="store_true", dest="as_json",
                   help="以 JSON 输出原始记录")
    a = p.parse_args(argv)

    from devkit import decision_log as _dlog
    records = _dlog.load(last_n=a.last)

    if not records:
        print("（决策日志为空 — 运行 `devkit auto` 后此处将出现记录）")
        return 0

    if a.as_json:
        print(_json.dumps(records, ensure_ascii=False, indent=2))
        return 0

    _OUTCOME_ICON = {
        "success": "✅", "failure": "❌", "skipped": "⏭",
        "cancelled": "🚫", "pending": "⏳",
    }
    print(f"{'时间':20}  {'任务':25}  {'分':4}  {'结果':6}  理由 / 备选")
    print("-" * 90)
    for r in records:
        ts = r.get("ts", "")[:19].replace("T", " ")
        tid = r.get("task_id", "?")[:24]
        score = r.get("score", "-")
        outcome = r.get("outcome", "?")
        icon = _OUTCOME_ICON.get(outcome, "?")
        reason = r.get("reason", "")[:38]
        alts = ", ".join(a["task_id"] for a in r.get("alternatives", [])[:3])
        alt_str = f"  [备选: {alts}]" if alts else ""
        print(f"{ts:20}  {tid:25}  {score:>4}  {icon:6}  {reason}{alt_str}")

    print(f"\n共 {len(records)} 条决策记录  (日志: devkit/decisions.jsonl)")
    return 0


def _cmd_setup(argv) -> int:
    """一键设置向导：引导填写 API key，写 .env，启动 LiteLLM 网关，验证可达性。"""
    p = argparse.ArgumentParser(prog="devkit setup", description="Loom 一键设置向导")
    p.add_argument("--non-interactive", action="store_true",
                   help="不提示用户输入（适合 CI / 测试）")
    a = p.parse_args(argv)

    from devkit.setup import run_setup
    result = run_setup(interactive=not a.non_interactive)

    for step in result["steps"]:
        icon = "✅" if step["ok"] else "❌"
        print(f"  {icon} {step['name']}: {step['message']}")

    if result["ok"]:
        print("\nLoom 环境就绪！运行示例：")
        print('  python3 -m devkit auto "帮我建一个 todo 应用" --yes --loop')
    else:
        print("\n部分步骤未完成，请检查上方错误信息后重试。")
    return 0 if result["ok"] else 1


def _auto_from_vision(a) -> int:
    """T13 Layer B 产品模式：从自然语言愿景生成 backlog 并运行第一个任务。"""
    import json as _json
    from devkit.rdloop import run_loop, ROOT
    from devkit.roles import load_stages
    from devkit import autoloop as _autoloop

    vision = a.vision
    print(f"🎯 Layer B 产品模式  vision={vision!r}")
    print("  Step 1: 规划 backlog…")

    # 使用 plan 阶段把愿景拆成任务清单
    all_stages = load_stages()
    by_key = {s.key: s for s in all_stages}
    plan_stage = by_key.get("plan")
    if not plan_stage:
        print("找不到 plan 角色（roles.yaml 配置问题）")
        return 1

    bl_path = pathlib.Path(a.backlog)
    plan_task = (
        f"把以下愿景拆分成具体可执行的开发任务清单，每个任务一行，按依赖顺序排列。\n\n"
        f"愿景：{vision}\n\n"
        f"只输出任务列表，每行格式：`[任务简短 id]: 任务描述`，不超过 10 个任务。"
    )
    result = run_loop(
        task=plan_task,
        stages=[plan_stage],
        run_id=f"auto-vision-{__import__('datetime').datetime.now().strftime('%Y%m%d-%H%M%S')}",
    )
    plan_file = pathlib.Path(result.get("run_dir", "")) / "01-plan.md"
    plan_text = plan_file.read_text(encoding="utf-8") if plan_file.exists() else ""

    # 解析任务列表 → backlog 格式
    import re
    backlog = []
    for line in plan_text.splitlines():
        m = re.match(r"\[([^\]]+)\]:\s*(.+)", line.strip())
        if not m:
            m = re.match(r"[-*]\s*(.+)", line.strip())
            if m:
                txt = m.group(1).strip()
                tid = re.sub(r"[^\w-]", "-", txt[:20]).lower()
                backlog.append({"id": tid, "task": txt, "status": "pending", "deps": []})
            continue
        backlog.append({"id": m.group(1).strip(), "task": m.group(2).strip(),
                        "status": "pending", "deps": []})

    if not backlog:
        print("  未能从愿景解析出任务清单（plan 阶段输出格式不符）")
        print(f"  plan 输出片段：{plan_text[:300]}")
        return 1

    backlog = _write_backlog(bl_path, backlog)
    print(f"  Step 2: 生成 {len(backlog)} 个任务 → {bl_path}")
    for i, t in enumerate(backlog, 1):
        print(f"    {i}. [{t['id']}] {t['task']}")

    if a.dry_run:
        print("（dry-run，未实际执行）")
        return 0

    # 运行第一个就绪任务（Loop B 模式用 --loop 持续运行）
    item = _autoloop.pick_next(backlog)
    if not item:
        print("  无就绪任务")
        return 0

    print(f"\n  Step 3: 运行第一个任务 [{item['id']}]…")
    run_args = _autoloop.run_once(item)

    for entry in backlog:
        if entry.get("id") == item.get("id"):
            entry["status"] = _autoloop.advance_state(entry.get("status","pending"), "start")
            from devkit import lease as _lease
            _lease.attach_lease(entry, owner_pid=os.getpid(), run_id=run_args["run_id"])
    backlog = _write_backlog(bl_path, backlog)

    r2 = run_loop(task=run_args["task"], run_id=run_args["run_id"])
    event = "success" if _autoloop.is_success_gate(r2.get("gate")) else "failure"
    for entry in backlog:
        if entry.get("id") == item.get("id"):
            entry["status"] = _autoloop.advance_state("running", event)
    backlog = _write_backlog(bl_path, backlog)
    print(f"✓ 任务 [{item['id']}] → {event}。用 `devkit auto --loop --yes` 继续运行剩余任务。")
    return 0 if event == "success" else 1


def _cmd_task(argv) -> int:
    """Task Center：跨 run 任务追踪（new / link / list / show / close）。"""
    from devkit import task_center as TC
    from devkit.rdloop import ROOT
    p = argparse.ArgumentParser(prog="devkit task", description="跨 run 任务追踪账本")
    sub = p.add_subparsers(dest="action")
    nw = sub.add_parser("new", help="新建任务")
    nw.add_argument("title", help="任务标题")
    nw.add_argument("--id", dest="task_id", help="指定任务 id（默认从标题自动生成）")
    lk = sub.add_parser("link", help="把 run 关联到任务")
    lk.add_argument("task_id", help="任务 id")
    lk.add_argument("run_id", help="run id（devkit/runs/<run-id>）")
    sub.add_parser("list", help="列出所有任务")
    sh = sub.add_parser("show", help="查看任务详情（含关联 runs）")
    sh.add_argument("task_id", help="任务 id")
    cl = sub.add_parser("close", help="关闭任务（标记为 closed）")
    cl.add_argument("task_id", help="任务 id")
    a = p.parse_args(argv)
    if not a.action:
        p.print_help()
        return 0
    if a.action == "new":
        try:
            t = TC.new_task(a.title, getattr(a, "task_id", None))
            print(f"✓ 新建任务：{t['id']}  {t['title']}")
        except ValueError as e:
            print(f"✗ {e}")
            return 1
        return 0
    if a.action == "link":
        if not (ROOT / "devkit" / "runs" / a.run_id).is_dir():
            print(f"找不到 run：{a.run_id}")
            return 1
        try:
            t = TC.link_run(a.task_id, a.run_id)
            print(f"✓ 已关联：{a.task_id} ← {a.run_id}  （共 {len(t['runs'])} 个 runs）")
        except KeyError:
            print(f"找不到任务：{a.task_id}")
            return 1
        return 0
    if a.action == "list":
        tasks = TC.list_tasks()
        if not tasks:
            print("暂无任务 —— 用 `devkit task new \"任务标题\"` 新建。")
            return 0
        print(f"{'id':<24}{'状态':<8}{'runs':>5}  标题")
        print("-" * 70)
        for t in tasks:
            print(f"  {t['id']:<22}{t.get('status','open'):<8}{len(t.get('runs',[])):>4}  {t['title'][:40]}")
        return 0
    if a.action == "show":
        t = TC.get_task(a.task_id)
        if t is None:
            print(f"找不到任务：{a.task_id}")
            return 1
        print(f"id:      {t['id']}\n标题：   {t['title']}\n状态：   {t.get('status','open')}\n创建：   {t.get('created','')}")
        runs = t.get("runs", [])
        if runs:
            from devkit import insight
            items = {it["run_id"]: it for it in insight.runs_list()}
            print(f"\n关联 runs（{len(runs)} 个）：")
            for rid in runs:
                gate = (items.get(rid, {}).get("gate") or "—")[:20]
                print(f"  {rid:<22}  {gate}")
        else:
            print("\n暂无关联 runs —— 用 `devkit task link` 关联。")
        return 0
    if a.action == "close":
        if TC.close_task(a.task_id):
            print(f"✓ 已关闭：{a.task_id}")
            return 0
        print(f"找不到任务：{a.task_id}")
        return 1
    return 0


def _cmd_asset(argv) -> int:
    """管理可复用资产（system prompt 片段 / 规则 / 技能）。"""
    from devkit import asset as A
    p = argparse.ArgumentParser(prog="devkit asset",
                                description="管理可复用资产（prompt / rule / skill / mcp）")
    sub = p.add_subparsers(dest="action")
    ls = sub.add_parser("list", help="列出所有资产")
    ls.add_argument("--type", dest="filter_type", metavar="TYPE",
                    help="只显示某类型（rule/skill/mcp/prompt）")
    ls.add_argument("--tag", dest="filter_tag", metavar="TAG", help="只显示含某 tag 的资产")
    sh = sub.add_parser("show", help="显示一个资产的完整内容")
    sh.add_argument("name", help="资产名称")
    ad = sub.add_parser("add", help="添加或覆盖一个资产")
    ad.add_argument("name", help="资产名称（唯一标识）")
    ad.add_argument("--type", dest="asset_type", default="prompt",
                    choices=["rule", "skill", "mcp", "prompt"], help="资产类型（默认 prompt）")
    ad.add_argument("--tags", default="", help="逗号分隔的 tag 列表")
    ad.add_argument("--content", required=True, help="资产内容")
    rm = sub.add_parser("remove", help="删除一个资产")
    rm.add_argument("name", help="资产名称")
    tr = sub.add_parser("trust", help="设置资产信任等级（0=未审查 … 6=系统内置）")
    tr.add_argument("name", help="资产名称")
    tr.add_argument("level", type=int, choices=range(7), metavar="LEVEL",
                    help="信任等级 0–6（0=untrusted 1=reviewed 2=reviewed+tested "
                         "3=trusted 4=verified 5=pinned 6=system）")
    a = p.parse_args(argv)
    if not a.action:
        p.print_help()
        return 0
    if a.action == "list":
        assets = A.load_assets()
        if not assets:
            print("暂无资产 —— 用 `devkit asset add` 添加。")
            return 0
        if a.filter_type:
            assets = [x for x in assets if x.get("type") == a.filter_type]
        if a.filter_tag:
            assets = [x for x in assets if a.filter_tag in x.get("tags", [])]
        print(f"{'名称':<24}{'类型':<10}{'信任':<14}{'tags'}")
        print("-" * 70)
        for x in assets:
            tags = ", ".join(x.get("tags", []))
            lvl = x.get("trust_level", 0)
            label = f"L{lvl}:{A.trust_label(lvl)}"
            print(f"  {x['name']:<22}{x.get('type','?'):<10}{label:<14}{tags}")
        print(f"\n共 {len(assets)} 个资产。")
        return 0
    if a.action == "show":
        x = A.get_asset(a.name)
        if x is None:
            print(f"找不到资产：{a.name}")
            return 1
        lvl = x.get("trust_level", 0)
        print(f"name:        {x['name']}\ntype:        {x.get('type','?')}\n"
              f"trust:       L{lvl} {A.trust_label(lvl)}\n"
              f"tags:        {', '.join(x.get('tags', []))}\n\n{x['content']}")
        return 0
    if a.action == "add":
        tags = [t.strip() for t in a.tags.split(",") if t.strip()]
        x = A.add_asset(a.name, a.asset_type, a.content, tags)
        print(f"✓ 已添加资产：{x['name']}（{x['type']}，trust=L{x['trust_level']}）")
        return 0
    if a.action == "remove":
        ok = A.remove_asset(a.name)
        if ok:
            print(f"✓ 已删除：{a.name}")
            return 0
        print(f"找不到资产：{a.name}")
        return 1
    if a.action == "trust":
        try:
            x = A.set_trust(a.name, a.level)
            print(f"✓ {x['name']} 信任等级 → L{a.level} {A.trust_label(a.level)}")
        except KeyError as e:
            print(f"✗ {e}")
            return 1
        except ValueError as e:
            print(f"✗ {e}")
            return 1
        return 0
    return 0


def _cmd_safety(argv) -> int:
    """扫描某次 run 的 build/ 产物，检查硬编码密钥 / shell 注入等安全问题。"""
    from devkit import safety
    from devkit.rdloop import ROOT
    p = argparse.ArgumentParser(prog="devkit safety",
                                description="安全扫描：对 build/ 产物检查密钥 / 注入等问题")
    p.add_argument("run_id", help="run id（devkit/runs/<run-id>）")
    p.add_argument("--runs-dir", default=str(ROOT / "devkit" / "runs"))
    a = p.parse_args(argv)
    build_dir = pathlib.Path(a.runs_dir) / a.run_id / "build"
    if not build_dir.is_dir():
        print(f"找不到 build 目录：{build_dir}")
        return 1
    r = safety.scan_build(build_dir)
    print(f"扫描 {r['scanned_files']} 个文件 · {r['rules_applied']} 条规则")
    if r["ok"]:
        print("✅ 无安全问题")
        return 0
    print(f"⚠️  发现 {len(r['violations'])} 处违规：\n")
    for v in r["violations"]:
        print(f"  [{v['rule']}] {v['file']}:{v['line']}  {v['snippet']}")
    return 1


def _cmd_recommend(argv) -> int:
    """推荐最适合当前任务的 backend（基于历史 model_fitness 数据）。"""
    from devkit import insight
    p = argparse.ArgumentParser(prog="devkit recommend",
                                description="根据历史数据推荐最适合当前任务的 backend")
    p.add_argument("task", help="任务描述（用于推断任务类型）")
    p.add_argument("--runs-dir", help="runs 目录路径")
    a = p.parse_args(argv)
    r = (insight.recommend_model(a.task, pathlib.Path(a.runs_dir))
         if a.runs_dir else insight.recommend_model(a.task))
    print(f"任务类型：{r['task_type']}")
    if r["backend"] is None:
        print("推荐：无历史数据，建议先用 deepseek（便宜）跑几次积累 runs。")
        print("提示：`devkit fitness` 查看积累后的分桶数据。")
    else:
        ok = f"{r['ok_rate']}%" if r["ok_rate"] is not None else "—"
        cost = f"${r['avg_cost']:.5f}" if r["avg_cost"] is not None else "—"
        print(f"推荐：{r['backend']}  （成功率 {ok} · 均成本 {cost} · 样本 {r['uses']} 次）")
        print(f"理由：{r['reason']}")
        print(f"\n用法：devkit \"任务\" --carrier implement={r['backend']}")
    return 0


def _cmd_runs(argv) -> int:
    """任务证据链视图：列表模式或详情模式（devkit runs / devkit runs <run-id>）。"""
    import re as _re
    from devkit import insight
    from devkit.rdloop import ROOT

    runs_dir = ROOT / "devkit" / "runs"

    # 如果第一个参数不以 - 开头且不是已知 flag，视为 run-id（详情模式）
    if argv and not argv[0].startswith("-"):
        run_id = argv[0]
        run_dir = runs_dir / run_id
        if not run_dir.is_dir():
            print(f"找不到 run：{run_id}")
            return 1
        task_snippet = gate = ""
        try:
            task_snippet = (run_dir / "00-task.md").read_text(encoding="utf-8").strip()[:120]
        except Exception:  # noqa: BLE001
            pass
        try:
            log = (run_dir / "run-log.md").read_text(encoding="utf-8")
            gm = _re.search(r"^(建议\s*GO[^\n]*|NO-GO[^\n]*)", log, _re.MULTILINE)
            gate = gm.group(1).strip() if gm else "—"
        except Exception:  # noqa: BLE001
            pass
        print(f"Run: {run_id}\n任务：{task_snippet}\nGate：{gate}\n")
        afs = sorted(run_dir.glob("*.artifact.json"))
        if afs:
            print(f"{'阶段':<12}{'载体':<18}{'tokens':>8}{'$':>10}{'verdict':<12}tests_passed")
            print("-" * 70)
            for af in afs:
                try:
                    import json as _json
                    a = _json.loads(pathlib.Path(af).read_text(encoding="utf-8"))
                    tok = str(a.get("tokens") or "—")
                    cost = f"${a['cost']:.5f}" if a.get("cost") is not None else "—"
                    print(f"{(a.get('stage') or '—'):<12}{(a.get('carrier') or '—'):<18}"
                          f"{tok:>8}{cost:>10}  {(a.get('verdict') or '—'):<10}  {a.get('tests_passed')}")
                except Exception:  # noqa: BLE001
                    print(f"  {af.name}  (解析失败)")
        build_dir = run_dir / "build"
        if build_dir.is_dir():
            files = [f.name for f in sorted(build_dir.rglob("*")) if f.is_file()
                     and not f.name.endswith(".pyc") and "__pycache__" not in str(f)]
            if files:
                print(f"\n物化文件：{', '.join(files)}")
        return 0

    # 列表模式（支持 --filter / --grep / --limit）
    p = argparse.ArgumentParser(prog="devkit runs", description="历史 runs 列表")
    p.add_argument("--filter", choices=["GO", "NO-GO"], dest="gate_filter",
                   help="只显示 GO 或 NO-GO 的 runs")
    p.add_argument("--grep", metavar="PATTERN", help="按任务描述过滤（大小写不敏感）")
    p.add_argument("--task-type", metavar="TYPE", help="只显示某类任务（feature/backend-fix/…）")
    p.add_argument("--limit", type=int, default=25, help="最多显示几条（默认 25）")
    a = p.parse_args(argv)

    items = insight.runs_list(runs_dir)
    if not items:
        print("还没有历史 runs —— 先 `devkit \"任务\"` 跑一遍。")
        return 0

    if a.gate_filter:
        keyword = "GO" if a.gate_filter == "GO" else "NO-GO"
        # 建议 GO → 包含 "GO"；NO-GO → 包含 "NO-GO"
        if a.gate_filter == "GO":
            items = [it for it in items if it.get("gate") and
                     "GO" in it["gate"] and "NO-GO" not in it["gate"]]
        else:
            items = [it for it in items if it.get("gate") and "NO-GO" in it["gate"]]
    if a.grep:
        pat = _re.compile(a.grep, _re.IGNORECASE)
        items = [it for it in items
                 if pat.search(it.get("task_snippet") or "") or pat.search(it["run_id"])]
    if a.task_type:
        items = [it for it in items if (it.get("task_type") or "") == a.task_type]

    if not items:
        print("没有匹配的 runs。")
        return 0

    print(f"{'run_id':<22}{'gate':<18}{'tokens':>9}{'$':>8}{'task_type':<12}任务")
    print("-" * 92)
    for it in items[:a.limit]:
        gate = (it["gate"] or "—")[:16]
        tok = str(it["tokens"]) if it["tokens"] is not None else "—"
        cost = f"${it['cost']:.3f}" if it["cost"] is not None else "—"
        tt = (it["task_type"] or "—")[:10]
        snippet = (it["task_snippet"] or "")[:40]
        print(f"{it['run_id']:<22}{gate:<18}{tok:>9}{cost:>8}  {tt:<10}{snippet}")
    if len(items) > a.limit:
        print(f"\n（共 {len(items)} 条，显示前 {a.limit} 条。用 --limit N 调整）")
    return 0


def _cmd_fitness(argv) -> int:
    """模型适配度：按任务类型分桶，看哪个后端最擅长哪类任务（backend-fix/feature/refactor/…）。"""
    from devkit import insight
    p = argparse.ArgumentParser(prog="devkit fitness",
                                description="按任务类型分桶的模型成功率/成本（历史 runs）")
    p.add_argument("--runs-dir")
    p.add_argument("--task-type", metavar="TYPE", help="只看某类任务（如 feature / backend-fix）")
    a = p.parse_args(argv)
    rep = (insight.model_fitness(pathlib.Path(a.runs_dir)) if a.runs_dir
           else insight.model_fitness())
    rows = rep["rows"]
    if a.task_type:
        rows = [r for r in rows if r["task_type"] == a.task_type]
    if not rows:
        print("还没有足够历史数据 —— 先跑几次 `devkit \"任务\"` 积累 runs。")
        if rep["task_types"]:
            print(f"现有任务类型：{', '.join(rep['task_types'])}")
        return 0
    print(f"{'后端':<12}{'任务类型':<16}{'次数':>5}{'成功率':>8}{'均$/次':>10}")
    for r in rows:
        ok = f"{r['ok_rate']}%" if r["ok_rate"] is not None else "—"
        cost = f"{r['avg_cost']:.5f}" if r["avg_cost"] is not None else "—"
        print(f"{r['backend']:<12}{r['task_type']:<16}{r['uses']:>5}{ok:>8}{cost:>10}")
    print(f"\n提示：多跑积累 runs，成功率越高的后端越适合该类任务。")
    return 0


def _cmd_recipes(argv) -> int:
    """列出内置的管道预设及其 stages/carriers 配置。"""
    from devkit.recipes import RECIPES, list_recipes
    p = argparse.ArgumentParser(prog="devkit recipes",
                                description="内置管道预设（--recipe NAME 直接使用）")
    p.add_argument("--json", action="store_true", help="以 JSON 格式输出")
    a = p.parse_args(argv)
    if a.json:
        import json as _json
        print(_json.dumps(RECIPES, ensure_ascii=False, indent=2))
        return 0
    print(f"{'名称':<25} {'stages':<40} 载体覆盖")
    print("-" * 80)
    for name in list_recipes():
        r = RECIPES[name]
        stages = ",".join(r["stages"])
        carriers = "  ".join(f"{k}={v}" for k, v in r.get("carriers", {}).items())
        print(f"  {name:<23} {stages:<40} {carriers}")
    print(f"\n用法：devkit \"任务\" --recipe <名称>  （可再加 --carrier / --cascade 覆盖）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
