# agent-fetch

用于 AI Agent 场景的 Go CLI：尽可能始终返回 Markdown 格式的网页内容。

**中文** | [English](./README.md)

## 背景

网页抓取结果通常是原始 HTML/JS/CSS，会给 LLM 带来大量噪音和 token 开销。这个工具封装了分级 fallback 流程，让 Agent 更稳定地拿到 Markdown 内容。

## 行为说明

`agent-fetch` 提供三种模式：

- `auto`（默认）：
  - 先用 `Accept: text/markdown` 请求
  - 若返回内容已判定为 Markdown，直接输出
  - 否则执行静态 HTML 正文抽取并转换为 Markdown
  - 若静态结果质量较低，再回退到无头浏览器渲染后转换
- `static`：只走静态路径，不启用浏览器回退
- `browser`：始终使用无头浏览器

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

## 使用方式

```bash
agent-fetch <url>
```

常用参数示例：

```bash
agent-fetch --mode auto --timeout 20s --browser-timeout 30s https://example.com
agent-fetch --mode browser --wait-selector 'article' https://example.com
agent-fetch --header 'Authorization: Bearer <token>' https://example.com
```

抓取成功内容输出到 `stdout`（Markdown）；错误信息输出到 `stderr`。

## 构建

```bash
go build -o agent-fetch ./cmd/agent-fetch
```

## 许可证

本项目基于 [MIT License](./LICENSE) 开源。
