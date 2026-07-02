"""devkit/decision_replay.py — 决策回放分析器（纯标准库）"""
import json
import os
from collections import Counter

def load_decisions(log_path=None):
    """读取 decisions.jsonl，不存在返回 []。"""
    if log_path is None:
        log_path = 'decisions.jsonl'
    if not os.path.exists(log_path):
        return []
    records = []
    with open(log_path, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            records.append(json.loads(line))
    return records

def outcome_stats(records):
    """统计决策结果：{total, success, failure, pending, by_carrier}。"""
    stats = {
        'total': len(records),
        'success': 0,
        'failure': 0,
        'pending': 0,
        'by_carrier': {},
    }
    carrier_counter = Counter()
    for rec in records:
        outcome = rec.get('outcome', 'pending')
        if outcome == 'success':
            stats['success'] += 1
        elif outcome == 'failure':
            stats['failure'] += 1
        else:
            stats['pending'] += 1
        carrier = rec.get('carrier')
        if carrier is not None:
            carrier_counter[carrier] += 1
    stats['by_carrier'] = dict(carrier_counter)
    return stats

def top_failing_tasks(records, n=5):
    """返回失败次数最多的任务列表 [{task_id, failures}]，按 failures 降序。"""
    failure_counter = Counter()
    for rec in records:
        if rec.get('outcome') == 'failure':
            task_id = rec.get('task_id')
            if task_id is not None:
                failure_counter[task_id] += 1
    return [
        {'task_id': tid, 'failures': cnt}
        for tid, cnt in failure_counter.most_common(n)
    ]

def summary_report(records):
    """生成摘要报告字符串。"""
    stats = outcome_stats(records)
    top = top_failing_tasks(records, n=1)
    if top:
        most_failing = top[0]['task_id']
        return (
            f"总计 {stats['total']} 条决策，"
            f"成功 {stats['success']}，"
            f"失败 {stats['failure']}，"
            f"最常失败：{most_failing}"
        )
    return (
        f"总计 {stats['total']} 条决策，"
        f"成功 {stats['success']}，"
        f"失败 {stats['failure']}"
    )
