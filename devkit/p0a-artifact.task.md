实现一个纯标准库的 Python 模块 `artifact.py`（Loom 的结构化产物总线 v1）。
只写这一个文件，不要新增依赖，不要写测试（测试由 Eval Gate 提供）。

## 模块要求

1. 模块级常量：
   `PROTECTED = ("contract", "failure", "patch_targets")`
   —— 这是"受保护字段"的名字元组（下游绝不压缩这些字段）。

2. `def make(stage, role, title, body, fields=None) -> dict`
   返回一个结构化产物 dict，形如：
   `{"stage": stage, "role": role, "title": title, "body": body, "fields": <dict>}`
   - `fields` 缺省为空 dict `{}`。
   - 传入 fields 时必须**拷贝**一份（`dict(fields)`），不能直接引用原对象（防止外部改动串改产物）。

3. `def extract_fields(stage, body) -> dict`
   从某阶段产物正文里抽取结构化字段：
   - 当 `stage == "implement"`：扫描 body 每一行，凡是形如 `# 路径/文件名.后缀` 的**注释行**（井号开头、内容是一个带后缀的文件路径），收集成列表，返回 `{"patch_targets": [那些路径]}`。例如 body 含一行 `# devkit/foo.py` → `{"patch_targets": ["devkit/foo.py"]}`。若一个都没有，返回 `{"patch_targets": []}`。
   - 当 `stage == "review"`：若 body 含 `"NO-GO"` 或 `"需要修改"`，返回 `{"failure": <包含该标记的第一行，strip 后>}`；否则返回 `{}`。
   - 其它 stage：返回 `{}`。

4. `def protected(artifact) -> dict`
   入参是 `make()` 产出的 dict。返回它 `fields` 里**只保留** key 在 `PROTECTED` 中的那些项，组成新 dict。
   例如 fields 为 `{"contract":"c","x":"y"}` → 返回 `{"contract":"c"}`。

## 风格
- 纯标准库（可用 `re`）。
- 函数短小、可读、加简短中文 docstring。
- 文件第一行用注释写出文件名：`# artifact.py`
