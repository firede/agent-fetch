<div align="center">

# agent-fetch

面向 LLM 的本地 Agent 物料管线。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go](https://github.com/firede/agent-fetch/actions/workflows/ci.yml/badge.svg)](https://github.com/firede/agent-fetch/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/firede/agent-fetch)](https://github.com/firede/agent-fetch/releases)<br/>
![Local-only](https://img.shields.io/badge/Local--only-Yes-emerald)
![Token-efficient](https://img.shields.io/badge/Token--efficient-Yes-emerald)
![Agent-ready](https://img.shields.io/badge/Agent--ready-Yes-emerald)

**中文** | [English](./README.md)

</div>

## 亮点

- **Markdown 优先的输出管线** -- 可读性算法抽取正文 + HTML 转 Markdown，Agent 直接拿到干净文本，无需处理 HTML/JS/CSS 噪音
- **无头浏览器回退** -- 自动渲染 JS 重度页面（SPA、动态仪表盘等），静态抽取无法满足时自动切换
- **自定义请求头** -- 支持 `Authorization`、`Cookie` 等任意 Header，轻松访问需认证的接口
- **多 URL 并发批量抓取** -- 一次传入多个 URL，并发请求，按输入顺序输出结构化结果

## 快速开始

```bash
# 安装
go install github.com/firede/agent-fetch/cmd/agent-fetch@latest

# 抓取页面
agent-fetch https://example.com
```

也可以从 [Releases](https://github.com/firede/agent-fetch/releases) 下载预编译二进制。

## 工作原理

在默认的 `auto` 模式下，agent-fetch 使用三级 fallback 管线：

```
发起请求，Accept: text/markdown
        |
        v
  响应是 Markdown？ --是--> 直接返回
        | 否
        v
  静态 HTML 正文抽取
  + Markdown 转换 --质量达标？--> 返回
        | 否
        v
  无头浏览器渲染
  + 抽取 + 转换 --> 返回
```

大多数页面无需启动浏览器即可处理，保持快速响应；遇到 JS 重度页面时自动切换到浏览器渲染。

## 模式

| 模式           | 行为                                                   | 需要浏览器         |
| -------------- | ------------------------------------------------------ | ------------------ |
| `auto`（默认） | 三级 fallback：原生 Markdown -> 静态抽取 -> 浏览器渲染 | 仅在静态质量不足时 |
| `static`       | 仅静态 HTML 抽取，不使用浏览器                         | 否                 |
| `browser`      | 始终使用无头 Chrome/Chromium                           | 是                 |
| `raw`          | 发送 `Accept: text/markdown`，原样返回 HTTP 响应体     | 否                 |

## 安装

### 从 Releases 下载

1. 在 [GitHub Releases](https://github.com/firede/agent-fetch/releases) 页面下载对应平台的压缩包。
2. 解压后为二进制添加执行权限：

```bash
chmod +x ./agent-fetch
```

3. 将二进制移动到 `PATH` 中的目录，或直接运行：

```bash
./agent-fetch https://example.com
```

#### macOS 提示

发布的二进制尚未进行 Apple 公证，Gatekeeper 可能阻止运行。移除 quarantine 属性即可：

```bash
xattr -dr com.apple.quarantine ./agent-fetch
```

### 使用 Go 安装

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@latest
```

安装指定版本：

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@v0.3.0
```

请确保 `$(go env GOPATH)/bin`（通常是 `~/go/bin`）已加入 `PATH`。

## 使用方式

```bash
agent-fetch [options] <url> [url ...]
agent-fetch web [options] <url> [url ...]
agent-fetch doctor [options]
```

### 参数

| 参数                | 默认值            | 说明                                                                                                               |
| ------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------------ |
| `--mode`            | `auto`            | 抓取模式：`auto` \| `static` \| `browser` \| `raw`                                                                 |
| `--format`          | `markdown`        | 输出格式：`markdown` \| `jsonl`                                                                                    |
| `--meta`            | `true`            | 附加 `title`/`description` 元数据（`markdown` 写入 front matter，`jsonl` 写入 `meta` 字段；`--meta=false` 可禁用） |
| `--timeout`         | `20s`             | HTTP 请求超时（适用于 static/auto 模式）                                                                           |
| `--browser-timeout` | `30s`             | 页面加载超时（适用于 browser/auto 模式）                                                                           |
| `--network-idle`    | `1200ms`          | 最后一次网络活动后等待多久再抓取页面内容                                                                           |
| `--wait-selector`   |                   | 等待指定 CSS 选择器出现后再抓取，如 `article`                                                                      |
| `--header`          |                   | 自定义请求头，可重复使用。如 `--header 'Authorization: Bearer token'`                                              |
| `--user-agent`      | `agent-fetch/0.1` | User-Agent 请求头                                                                                                  |
| `--max-body-bytes`  | `8388608`         | 最大响应读取字节数                                                                                                 |
| `--concurrency`     | `4`               | 多 URL 请求时的最大并发数                                                                                          |
| `--browser-path`    |                   | 为 `browser` / `auto` 模式指定浏览器可执行文件路径或名称                                                           |

### 示例

```bash
# 默认 auto 模式
agent-fetch https://example.com

# 强制浏览器渲染 JS 重度页面
agent-fetch --mode browser --wait-selector 'article' https://example.com

# 指定浏览器二进制（容器/自定义安装场景常用）
agent-fetch --mode browser --browser-path /usr/bin/chromium https://example.com

# 静态抽取，不带 front matter
agent-fetch --mode static --meta=false https://example.com

# 获取原始 HTTP 响应体
agent-fetch --mode raw https://example.com

# 带认证请求
agent-fetch --header "Authorization: Bearer $TOKEN" https://example.com

# 批量抓取，控制并发
agent-fetch --concurrency 4 https://example.com https://example.org

# 结构化 JSONL 输出
agent-fetch --format jsonl https://example.com

# 检查环境可用性
agent-fetch doctor

# 使用指定浏览器路径进行环境检查
agent-fetch doctor --browser-path /usr/bin/chromium
```

## 多 URL 批量抓取（Markdown）

传入多个 URL 时，请求会并发执行（通过 `--concurrency` 控制），按输入顺序输出，使用任务标记区分各结果：

```text
<!-- count: 3, succeeded: 2, failed: 1 -->
<!-- task[1]: https://example.com/hello -->
...markdown...
<!-- /task[1] -->
<!-- task[2](failed): https://abc.com -->
<!-- error[2]: ... -->
```

退出码：全部成功为 `0`，部分或全部失败为 `1`，参数/用法错误为 `2`。

## JSONL 输出约定

当使用 `--format jsonl` 时，每个任务输出一行 JSON（不输出汇总行）：

```json
{"seq":1,"url":"https://example.com","resolved_mode":"static","content":"...","meta":{"title":"...","description":"..."}}
{"seq":2,"url":"https://bad.example","error":"http request failed: timeout"}
```

字段说明：

- `url`：输入 URL
- `resolved_url`：仅在与 `url` 不同时输出
- `resolved_mode`：`markdown`、`static`、`browser`、`raw` 之一
- `meta`：仅在 `--meta=true` 且存在元数据时输出

## Agent 集成

项目附带一份 [SKILL.md](./skills/agent-fetch/SKILL.md)，可供支持 skill 文件的编程 Agent 使用。将 skill 目录指向 `skills/agent-fetch`，Agent 即可在内置抓取能力不足时调用 `agent-fetch`。

`agent-fetch` 从命令行读取参数、向 stdout 输出结果（`markdown` 或 `jsonl`），可以轻松集成到任意 Agent 管线或基于 shell 的工具调用：

```bash
result=$(agent-fetch --mode static https://example.com)
```

## 什么场景需要这个工具？

下表将 agent-fetch 与部分编程 Agent 内置的网页抓取能力做对比。各产品的内置能力因版本而异。

| 场景                                 | 内置 web fetch |            agent-fetch             |
| ------------------------------------ | :------------: | :--------------------------------: |
| 基础页面抓取 + HTML 简化             |      支持      |                支持                |
| JS 渲染页面（SPA）                   |   取决于产品   |         支持（无头浏览器）         |
| 自定义请求头（认证、Cookie）         |   取决于产品   |         支持（`--header`）         |
| 不做 AI 摘要（直接输出抽取到的正文） |   取决于产品   | 支持（受 `--max-body-bytes` 限制） |
| 批量并发抓取多个 URL                 |   取决于产品   |      支持（`--concurrency`）       |
| CSS 选择器等待/抽取                  |   取决于产品   |     支持（`--wait-selector`）      |
| 在编程 Agent 之外使用（CLI、CI/CD）  |     不适用     |          支持（独立 CLI）          |

**内置 web fetch 的典型工作方式：** Claude Code 的 WebFetch、Codex 的内置抓取等工具，通常通过 HTTP 请求获取页面，将 HTML 转换为 Markdown，然后由 AI 模型对内容进行摘要或截断以适应上下文窗口。这一流程速度快、能覆盖大多数页面，但通常不执行 JavaScript（SPA 等 JS 渲染页面可能返回不完整的内容）、不支持自定义请求头，且一次只处理单个 URL。

- **没有内置 web fetch 工具**（其他 Agent 框架、CLI 管线、CI/CD）-- 直接用 agent-fetch 作为主力抓取工具。
- **有内置 web fetch 工具** -- 用 agent-fetch 补充处理 JS 重度页面、认证接口、批量抓取，或需要未经摘要的原始抽取内容的场景。

## 运行时依赖

`browser` 和 `auto` 模式可能需要宿主机上安装 Chrome 或 Chromium。

使用 `--mode static` 或 `--mode raw` 可完全避免浏览器依赖。

- 可以运行 `agent-fetch doctor` 检查运行时/浏览器可用性，并在浏览器模式不可用时获得修复建议。
- 当浏览器安装在非默认位置（例如容器镜像内自定义路径）时，使用 `--browser-path` 指定可执行文件。

## 构建

```bash
go build -o agent-fetch ./cmd/agent-fetch
```

## 许可证

本项目基于 [MIT License](./LICENSE) 开源。
