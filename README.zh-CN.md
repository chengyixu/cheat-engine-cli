<p align="center">
  <img src="docs/assets/cecli-mark.svg" width="112" height="112" alt="Cheat Engine CLI 标志">
</p>

<h1 align="center">Cheat Engine CLI</h1>

<p align="center"><strong>原生 macOS 与 Windows 进程内存工作流，并兼容远程 Cheat Engine <code>ceserver</code>。</strong></p>

<p align="center">
  <a href="README.md">English</a> ·
  <a href="README.zh-CN.md">简体中文</a> ·
  <a href="https://github.com/chengyixu/cheat-engine-cli-skill">Agent Skill</a>
</p>

<p align="center">
  <a href="https://github.com/chengyixu/cheat-engine-cli/actions/workflows/cli.yml"><img alt="CLI workflow" src="https://github.com/chengyixu/cheat-engine-cli/actions/workflows/cli.yml/badge.svg"></a>
  <a href="https://github.com/chengyixu/cheat-engine-cli/releases"><img alt="GitHub release" src="https://img.shields.io/github/v/release/chengyixu/cheat-engine-cli?display_name=tag&sort=semver"></a>
  <a href="https://github.com/chengyixu/cheat-engine-cli"><img alt="Go 1.24+" src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://chengyixu.github.io/cheat-engine-cli/"><img alt="Documentation" src="https://img.shields.io/badge/docs-GitHub%20Pages-222?logo=github"></a>
</p>

> [!IMPORTANT]
> **独立项目。** `cecli` 不是 Cheat Engine 官方命令行版本。它与上游 `ceserver` 协议互操作，并在仓库中保留原始 Cheat Engine 源码以便追溯。重新分发前请阅读[声明与许可状态](NOTICE.md)。

Cheat Engine CLI（`cecli`）是一个跨平台、JSON-first 的命令行接口，用于已获授权的进程检查和内存工作流。在 macOS 上运行 `cecli --native` 可检查 macOS 进程，在 Windows 上运行同一参数可检查 Windows 进程。连接另一台机器或虚拟机中的上游 Cheat Engine `ceserver` 或可选 `cebridge` 时使用 `--endpoint`。

## 为什么选择 `cecli`？

| 需求 | Cheat Engine GUI | `cecli` |
|---|---:|---:|
| 交互式探索 | 优秀 | 专注 |
| Shell 脚本与 CI | 有限 | 原生支持 |
| 稳定 JSON 输出 | 无 | 默认模式 |
| 本机 macOS/Windows 内存 | 平台 GUI | `--native` |
| 远程 Linux/Android 目标 | 通过 `ceserver` | 通过 `ceserver` |
| AI Agent 集成 | 需要 UI 自动化 | 内置规则、技能和结构化错误 |
| 安全的非交互写入 | 手工流程 | `--dry-run`、`--yes`、`--verify` |

## 核心能力

- **兼容上游协议** — 数据包布局来自 `Cheat Engine/ceserver/ceserver.h`、`api.h` 和 `networkInterface.pas`。
- **原生本机内存访问** — macOS 与 Windows 上的进程发现、内存区域、读取、精确扫描和受保护写入由 `--native` 在进程内完成。
- **进程清单** — 列出进程、模块、线程、架构和已映射内存区域。
- **类型化内存读取** — 解码有符号/无符号整数、浮点数、UTF-8、UTF-16LE、十六进制或 base64。
- **可移植精确扫描器** — 扫描带 `??` 通配符的字节模式或已编码值，并限制对齐、保护属性和结果数。
- **受保护内存写入** — 缺少 `--yes` 时默认拒绝；用 `--dry-run` 在不连接的情况下预览；用 `--verify` 回读验证。
- **高级目标控制** — 以相同确认契约分配/释放内存、修改页面保护、加载目标模块并调整速度。
- **有边界的调试工作流** — 收集有限数量的调试事件，管理活动会话硬件断点，读取或替换经过验证的原始线程上下文。
- **远程管理** — 查看服务端路径/选项，并以有界传输管理已授权目标文件。
- **AI-native 命令契约** — 默认 JSON、stdout 只输出数据、stderr 输出结构化错误、稳定退出码、类型化帮助、字段投影、静默模式、`--brief`、Agent 规则和本地反馈。
- **快速单文件** — 标准库 Go 实现，无运行时依赖。

## 给 Agent 一套完整操作手册

配套的 [Cheat Engine CLI Skill](https://github.com/chengyixu/cheat-engine-cli-skill) 把命令集合转化为严格的 Agent 流程：确认授权、发现目标、查看映射、读取、扫描、预览、确认、验证和恢复。它同时提供 English 与简体中文指引，覆盖 macOS 本机、Windows 本机和远程 `ceserver`。

```bash
npx skills add chengyixu/cheat-engine-cli-skill@cheat-engine-cli -g -y
```

Skill 会把本仓库作为 CLI 实现和命令参考的唯一规范来源。

## 快速开始

### 1. 构建 CLI

```bash
git clone https://github.com/chengyixu/cheat-engine-cli.git
cd cheat-engine-cli
make build
./bin/cecli --human --help
```

### 2A. 检查本机 macOS 或 Windows 目标

Windows 正常构建后，在本机进程和内存命令中添加 `--native`。macOS 的 Mach task API 还需要带调试权限签名的 CLI；辅助脚本会选择已安装的 Apple Development 或 Developer ID 证书：

```bash
# macOS
make sign-macos-native
./bin/cecli --native server info --human
./bin/cecli --native process list --filter game --human

# Windows PowerShell
go build -o bin/cecli.exe ./cmd/cecli
./bin/cecli.exe --native process list --filter game --human
```

macOS 仍可能拒绝 SIP 保护或其他受限进程。当目标进程处于更高完整性级别时，Windows 可能需要提升终端权限。

原生模式目前覆盖服务身份/路径、进程发现、架构、内存区域、有界读取、可移植精确扫描和确认门控写入。远程文件、管道、模块、线程、调试器会话、内存分配和保护修改仍只支持 `ceserver`。

### 2B. 连接远程目标

在已获授权的 Linux 或 Android 目标上构建并运行上游服务：

```bash
cd "Cheat Engine/ceserver/gcc"
make
sudo ./ceserver
```

上游默认端口为 `52736`。请使用主机防火墙或带认证的隧道限制网络访问。

### 3. 检查目标

```bash
# 验证连接目标；本机运行时添加 --native
./bin/cecli server info --human

# 查找目标 PID
./bin/cecli process list --filter game --human

# 查看内存映射
./bin/cecli memory regions --pid 4242 --human

# 读取浮点数
./bin/cecli memory read --pid 4242 --address 0x7FF612340120 --format typed --type f32

# 扫描 32 位整数
./bin/cecli memory scan --pid 4242 --type i32 --value 100 --alignment 4 --protection writable
```

## 安全写入流程

```bash
# 预览：不连接服务，也不修改目标
./bin/cecli memory write \
  --pid 4242 \
  --address 0x7FF612340120 \
  --type i32 \
  --value 999 \
  --dry-run

# 检查 PID、地址和编码字节并明确确认后再执行
./bin/cecli memory write \
  --pid 4242 \
  --address 0x7FF612340120 \
  --type i32 \
  --value 999 \
  --yes \
  --verify
```

`cecli` 不会交互询问缺失参数。写入命令缺少 `--yes` 时会以退出码 `2` 结束，并在 stderr 输出机器可读错误。

## JSON-first 输出

```bash
cecli process list | jq '.data.processes[] | select(.name | test("game"; "i"))'
```

Agent 模式下的成功响应包含 `ok`、`command`、`data`、`meta`、`rules`、`skills` 和 `issue`。错误只写入 stderr。

Agent 可以请求单个命令契约、只选择需要的字段，或者在只关心退出码时关闭全部输出：

```bash
cecli memory scan --help --pretty | jq '.data.commands[0]'
cecli version --fields name,version
cecli self-check --quiet
```

机器可读帮助包含 `schema_version`、CLI 版本、参数类型、默认值、枚举、条件约束以及成功/错误 envelope schema。`--fields` 支持点路径，并通过 `meta.missing_fields` 报告不存在的选择；`--quiet` 同时抑制 stdout 与 stderr，但不改变退出码。

## 常用命令

| 命令 | 用途 |
|---|---|
| `cecli server info` | 显示协议版本、服务名、ABI 和端点。 |
| `cecli process list` | 列出并筛选可见进程。 |
| `cecli process info --pid` | 显示架构及模块/线程数量。 |
| `cecli module list --pid` | 列出已映射模块和地址。 |
| `cecli thread list --pid` | 列出目标线程 ID。 |
| `cecli debug trace --pid` | 在有限事件会话中附加、继续并分离。 |
| `cecli memory regions --pid` | 枚举内存区域和保护属性。 |
| `cecli memory read --pid --address` | 读取原始或类型化内存。 |
| `cecli memory scan --pid` | 扫描精确字节或类型化数值。 |
| `cecli memory write --pid --address` | 预览或执行受保护写入。 |
| `cecli memory alloc|free|protect` | 管理目标内存分配和页面保护。 |
| `cecli remote ls|stat|get|put|mkdir|rm|chmod` | 检查或修改已授权目标文件系统。 |
| `cecli pipe open|read|write|close` | 通过 `ceserver` 命名管道交换有界数据。 |
| `cecli symbol list --path` | 读取并筛选压缩 ELF 符号。 |
| `cecli skills [name]` | 查看内置 Agent 工作流。 |
| `cecli issue create|list|show|transition` | 记录结构化本地反馈。 |
| `cecli completion bash|zsh|fish` | 生成 Shell 补全。 |
| `cecli self-check` | 验证内置契约和本地状态。 |

完整内容参阅[命令参考](docs/command-reference.md)和[架构说明](docs/architecture.md)。

## 配置

优先级依次为命令行参数、环境变量、默认值。

| 设置 | 参数 | 环境变量 | 默认值 |
|---|---|---|---|
| 本机原生目标 | `--native` | `CECLI_NATIVE=true` | 关闭 |
| `ceserver` 端点 | `--endpoint` | `CECLI_ENDPOINT` | `127.0.0.1:52736` |
| 连接名称 | `--connection-name` | `CECLI_CONNECTION_NAME` | 未设置 |
| 网络超时 | `--timeout` | `CECLI_TIMEOUT` | `30s` |
| 输出模式 | `--human` / `--agent` | `CECLI_OUTPUT=human` | JSON |
| JSON 字段选择 | `--fields path,...` | `CECLI_FIELDS` | 全部命令数据 |
| 静默执行 | `--quiet` / `-q` | — | 关闭 |
| 本地 issue 状态 | — | `CECLI_STATE_DIR` | 平台用户配置目录 |

## 构建与测试

```bash
make check              # go test ./... + go vet ./...
make race               # race detector
make smoke              # 构建 + JSON 契约冒烟测试
make sign-macos-native  # 构建并以调试权限签名 macOS CLI
make snapshot           # 本地 GoReleaser 快照
make snapshot-darwin    # 启用 CGO 的 arm64 + x86_64 macOS 压缩包
```

Darwin Release 压缩包在 macOS runner 上启用 CGO 构建，并包含 `scripts/sign-macos-native.sh`。签名必须由用户或分发者在本机完成，因为调试权限二进制需要合适的本地 Apple 签名身份。

测试套件使用假的 TCP `ceserver` 验证进程/模块/线程枚举、区域、读取、写入、客户端/服务端扫描、连接命名、服务终止、调试事件、断点、上下文、扩展控制、命名管道、远程文件和压缩符号的数据包宽度与响应。单元测试不需要高权限真实进程。

## 当前范围

远程模式覆盖捆绑上游 `ceserver` 分发的所有非废弃命令，包括连接命名、受保护远程终止和服务端 AOB 数据包。macOS 与 Windows 原生模式覆盖发现、区域、读取、可移植精确扫描和受保护写入所需的核心进程内存操作。更安全的可移植扫描器仍是默认选择；`memory aobscan` 只用于明确的远程数据包兼容测试，并要求 `--yes`。

## 负责任使用

只能在自己拥有或已明确获授权的系统、应用、游戏和进程上使用。进程内存可能包含凭据、个人信息、加密材料和专有数据。不要把 `ceserver` 暴露在不可信网络上。

## 常见问题

### Cheat Engine CLI 是什么？

Cheat Engine CLI 是独立命令行接口，提供 macOS 和 Windows 本机进程内存访问，并兼容上游 Cheat Engine `ceserver` 协议。它面向已授权的调试、兼容性测试、模组开发、研究、CI 和 AI Agent 工作流。

### `cecli` 会取代 Cheat Engine GUI 吗？

不会。GUI 更适合交互式反汇编、表格编辑、调试器操作和可视化探索。`cecli` 专注可确定的本机或远程检查、精确扫描、类型化读取、受保护写入和自动化。

### 不启动 `ceserver` 也能在本机使用吗？

可以，macOS 和 Windows 均支持。在相同操作系统上添加 `--native` 即可检查符合条件的进程。远程 Linux 与 Android 仍使用 `ceserver`；原生 Linux 支持仍在路线图中。

### 默认允许写内存吗？

不允许。缺少 `--yes` 时命令会拒绝写入；`--dry-run` 不连接、不修改；`--verify` 会比较回读字节。

### 这是 Cheat Engine 官方项目吗？

不是。这是基于上游协议兼容性的独立项目。

## 路线图

- Linux 本机进程后端。
- 未知初始值及 changed/unchanged 细化扫描会话。
- 指针扫描流程和可复用地址表。
- 符号名查找和模块相对地址表达式。
- 架构感知寄存器解码和一键断点跟踪编排。
- 完成仓库许可审查后的 MCP schema 导出与包管理器分发。

## 上游归属

本仓库起始于 [cheat-engine/cheat-engine](https://github.com/cheat-engine/cheat-engine) 的完整克隆。上游将 Cheat Engine 描述为专注个人游戏和应用模组的开发环境。原始源码、历史和捆绑组件声明均保留在仓库中。

## 声明

克隆的上游源码树目前没有检测到覆盖整个仓库的统一许可证。重新分发前请阅读 [NOTICE.md](NOTICE.md)。安全说明位于 [SECURITY.md](SECURITY.md)。
