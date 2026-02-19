# agent-fetch

用于 AI Agent 场景的 Go CLI：尽可能始终返回 Markdown 格式的网页内容。

**中文** | [English](./README.md)

## 背景

网页抓取结果通常是原始 HTML/JS/CSS，会给 LLM 带来大量噪音和 token 开销。这个工具封装了分级 fallback 流程，让 Agent 更稳定地拿到 Markdown 内容。

如果你使用 Codex、Claude Code 等工具，需要注意它们可能已内置 HTML 简化/抓取能力。是否仍需要 `agent-fetch`，应根据你的场景判断。

## 行为说明

`agent-fetch` 提供四种模式：

- `auto`（默认）：
  - 先用 `Accept: text/markdown` 请求
  - 若返回为 `text/markdown` 或内容已判定为 Markdown，直接输出
  - 否则执行静态 HTML 正文抽取并转换为 Markdown（默认注入 `title`/`description` front matter）
  - 若静态结果质量较低，再回退到无头浏览器渲染后转换（默认也注入 front matter）
- `static`：只走静态路径，不启用浏览器回退
- `browser`：始终使用无头浏览器
- `raw`：带 `Accept: text/markdown` 发起一次 HTTP 请求，并将该响应体原样输出（不做回退/转换）
- `--meta`（默认 `true`）：控制非 `raw` 输出是否附带 front matter（`title`/`description`）。对于 `auto`/`static` 的直返 markdown，可能会额外发起一次 HTML 请求用于补齐元信息。

## 运行时依赖

`browser` 模式要求宿主机可用 Chrome/Chromium 浏览器。

`auto` 模式可能回退到浏览器渲染，因此在部分页面上也会依赖 Chrome/Chromium。

如需避免浏览器依赖，请使用 `--mode static` 或 `--mode raw`。

## 安装（使用 Go）

如果本地已经安装 Go，可直接执行：

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@latest
```

安装指定版本：

```bash
go install github.com/firede/agent-fetch/cmd/agent-fetch@v0.2.0
```

请确保 `$(go env GOPATH)/bin`（通常是 `~/go/bin`）已加入 `PATH`。

## 安装（从 Releases 下载）

1. 在 [GitHub Releases](https://github.com/firede/agent-fetch/releases) 页面下载对应平台的压缩包。
2. 解压后为二进制添加执行权限：

```bash
chmod +x ./agent-fetch
```

### macOS 提示

当前发布的二进制尚未进行 Apple 公证（暂未接入 Apple Developer 公证流程），因此 Gatekeeper 可能提示：

`“agent-fetch”未打开。Apple 无法验证“agent-fetch”是否包含可能危害 Mac 安全或泄漏隐私的恶意软件。`

用于本地验证时，可先移除 quarantine 属性再运行：

```bash
xattr -dr com.apple.quarantine ./agent-fetch
./agent-fetch https://example.com
```

## Agent Skills

Skill 位置：[`skills/agent-fetch`](./skills/agent-fetch/SKILL.md)。

其中包含 `agent-fetch` 的安装指引和使用说明。

## 使用方式

```bash
agent-fetch <url>
```

常用参数示例：

```bash
agent-fetch --mode auto --timeout 20s --browser-timeout 30s https://example.com
agent-fetch --mode browser --wait-selector 'article' https://example.com
agent-fetch --mode static --meta=false https://example.com
agent-fetch --mode raw https://example.com
agent-fetch --header 'Authorization: Bearer <token>' https://example.com
```

抓取成功内容输出到 `stdout`（`raw` 模式为单次 HTTP 响应体原样输出）；错误信息输出到 `stderr`。

## 构建

```bash
go build -o agent-fetch ./cmd/agent-fetch
```

## 许可证

本项目基于 [MIT License](./LICENSE) 开源。
