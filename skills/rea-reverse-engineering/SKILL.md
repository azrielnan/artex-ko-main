---
name: rea-reverse-engineering
description: Use the REA MCP to analyze user-provided binaries, applications, packages, source trees, and runtime evidence. Use in ARTEX's isolated reverse-analysis mode.
mcps: REA
---

# REA 逆向分析

在 ARTEX 的「逆向分析」模式中，结合官方 REA MCP 分析用户提供的程序材料。该模式使用独立 Agent 和独立会话记录，不读取安全测试模式的对话、任务、资产、流量或发现。

官方项目与中文说明：<https://github.com/morluto/rea> · <https://github.com/morluto/rea/blob/main/docs/readme/README_zh.md>

## 准备

- ARTEX 预置名为 `REA` 的 stdio MCP 配置，默认关闭且只对逆向 Agent 可见。GitHub Release 分发包会优先使用 `runtime/rea` 中随包提供的 Node.js 22.19.0、REA 6.5.0 和 npm 运行依赖；源码开发环境没有该目录时才回退到 `npx -y rea-agents@6.5.0 mcp`。
- 在系统设置的 MCP 管理中启用 `REA`。REA 使用的具体分析器还可能需要按官方文档另行安装和配置。
- 第三方 REA、Node.js 及其依赖的许可证和声明随发布包提供；使用前请查看上游仓库和压缩包中的 LICENSE/NOTICE 文件，并遵守各分析器自己的许可与运行要求。

## 工作流程

1. 明确用户提供的文件、包、工程或既有运行时材料，并确认要回答的具体问题；不可把本地分析对象默认为网络测试目标。
2. 先检查 REA MCP 实际公开的工具、提示与资源，再根据材料类型选择工具。不得猜测工具名、参数或成功结果。
3. 从整体结构开始，沿问题追踪模块、函数、调用关系、导入、字符串、资源和引用。比较版本前核对输入身份与范围。
4. 用工具结果中的证据定位结论，注明路径、地址/偏移、符号或观察来源，并区分直接观察、推断、未知和矛盾。未覆盖区域不得据此断言不存在。
5. 不修改原始样本；生成文件写入当前会话工作目录。没有用户明确要求时，不启动样本、不附加进程、不连接设备、不改变目标状态。
6. 工具或分析器不可用时，指出缺少的依赖或能力并说明静态替代方案；不要擅自安装分析器或改动宿主环境。
7. 先给结论，再给关键证据、限制和建议的下一步。避免把未经证实的猜测写成事实。

## 适用范围

适用于本地程序结构、实现逻辑、兼容性和既有运行证据的分析。网络渗透、目标扫描、漏洞利用、凭据窃取及许可/DRM 绕过不属于本技能的逆向分析工作流。
