# Complexity signal

非阻断的增量复杂度信号。目标是让 LLM 在 commit / push 时看到“本次改动是否让某个函数变得更复杂”，而不是把全仓历史债务变成每次都出现的噪音。

## 运行方式

```bash
python3 scripts/complexity.py --staged   # pre-commit：本次暂存文件
python3 scripts/complexity.py --push     # pre-push：当前分支相对 upstream 的改动
python3 scripts/complexity.py --audit    # 手动全量审计，不挂 hook
```

`HDD_COMPLEXITY=0` 可跳过。脚本的任何工具缺失、网络失败或解析错误都只打印 `[complexity] scan skipped`，最终 `exit 0`，不阻止提交 / push。

## 增量口径

- `--staged`：`git diff --cached` 的源码文件；与 `HEAD` 版本对比函数复杂度。
- `--push`：`git diff <merge-base(upstream, HEAD)> HEAD` 的源码文件；没有 upstream 时退回 `origin/main` / `HEAD^`。
- `--audit`：`git ls-files` 全量扫描，仅供人工审计。

Go / Python 是**函数级**对比：只有以下情况会出现在报告里：

- 新函数且超过阈值；
- 已有函数复杂度相对 base **上升**且超过阈值；
- 已有函数从阈值下跨到阈值上。

已有且复杂度不变 / 下降的函数不报告。TS 的 ESLint `complexity` 没有稳定函数 ID，因此只报告修改文件里超过阈值的函数；当前基线没有 >20 的函数，等同于增量。

## 阈值

| 语言 | 指标 | 阈值 |
| --- | --- | ---: |
| Go | `gocyclo` 圈复杂度 | >15 |
| Go | `gocognit` 认知复杂度 | >15 |
| Python | `radon cc` | >20 |
| TypeScript | ESLint `complexity` | >20 |

排除测试、生成物、`.scratch/`、`recognizer/tools/`、`scripts/`、`node_modules`、`.venv`。

## 输出

stdout 只打印摘要：

```text
[complexity] staged since HEAD: 2 file(s), 2 finding(s)
  go         handleCreateAnalysis  cmd/hdd/serve_analysis.go 42 (31→42) [increased]
  python     _extract_items        recognizer/postprocess.py 98 (—→98) [new]
[complexity] detail: <git-dir>/hdd-complexity/report.md
```

详细报告（Markdown + JSON）写到：

```bash
$(git rev-parse --git-dir)/hdd-complexity/report.md
$(git rev-parse --git-dir)/hdd-complexity/report.json
```

不会进入版本库。

## LLM 行动规则

看到 `new` 或 `increased` 时：

1. 读取报告里的文件、函数和 base → current；
2. 判断是否由本次改动引入，能否低成本拆分；
3. 能拆就拆；不拆则在 commit message 或回复里说明理由（例如规则引擎的固有分支、性能路径、后续单独处理）。

存量函数（未修改、复杂度未上升）不要求处理；需要全量盘点时手动跑 `--audit`。

## 工具与回退

- Go：优先使用 `gocyclo` / `gocognit`，否则 `go run <module>@<pinned version>`。
- Python：`uvx radon cc -j`。
- TS：`apps/mobile/node_modules/.bin/eslint`；依赖未安装时跳过。
- 所有工具失败都不会阻断提交。

## 已知限制

- `--staged` 读取工作区文件内容；如果同一个文件存在未暂存改动，信号会包含这些改动。对非阻断提示可接受。
- TS 只能按修改文件报告，不做函数级 base 对比；当前阈值下基线为空，噪音很小。
- 阈值是启发式，不是质量门禁。复杂度下降或不变的存量函数不应为了“清零”而重构。
