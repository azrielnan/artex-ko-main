# ARTEX 当前实现与优化报告

## 结论

当前工程仍保持单体 Go 后端、内嵌 Next.js 前端、PostgreSQL、planner/worker 和审批拦截链路。参考 [artex-ymh](https://github.com/panda0x123/artex-ymh)（本地核对版本 `48adc7f`）的做法，新增能力被拆成独立包和独立 API，避免把会话、隧道、蜜罐和知识库逻辑塞进原有资产处理器。

已经真正接入运行链的模块包括：知识库、CVE 表、内网会话/凭据台账、WebShell 会话工具、session_recon、内置 TCP 转发、配置后可运行的 Chisel/suo5 进程生命周期、Chains 场景注入、蜜罐静态识别、stage、tools manifest 和 doctor。侧栏页面只是入口，数据最终通过 API 和 PostgreSQL 读写。

## 与参考项目的对应关系

| 参考项目能力 | 当前工程状态 | 说明 |
| --- | --- | --- |
| `honeydetect/` | 已接入 | 资产元数据静态匹配，结果写入 `honeypot_assessments`，页面可按任务执行分析。 |
| `session/` | 已接入 Server | `register_session` 会探测并加密落库，`session_exec/read/write` 通过 Registry 执行，`session_recon` 回写资产；反弹 Shell 仍需目标侧协议适配。 |
| `stage/` | 已接入 | 用于受控暂存，带 TTL、一次性下载和审计。 |
| `tools/`、`packaging/` | 已接入 | 可执行工具清单和校验基础已存在。 |
| `tunnel/` | 已接入生命周期适配层 | 内置 TCP 转发可直接运行；Chisel/suo5 在填写 command、回环 listen_addr、配置二进制并批准后由 Server 启动真实进程、检查存活并拆除。 |
| Chains | 已接入 worker | 按意图注入九类场景问题、反证和转向建议。 |
wq
## CVE 问题与修复方案

原实现的问题是：更新按钮直接依赖 NVD 在线 API，长时间范围被拆成很多请求，接口超时或限流时页面只显示空结果；前端也没有把错误显示出来。

现在的策略是：

1. 近 30 天继续使用 NVD 2.0 API，适合增量更新。
2. 近十年完整年度数据使用 NVD 官方年度 `.json.gz` Feed，逐条流式解压、解析、去重并写入 `cve_records`；Feed 缓存在 `data/cve-cache/`。
3. `cve_id` 唯一约束和 `UpsertCVE` 保证重复导入不会产生重复记录。
4. 前端显示后端错误，避免失败时伪装成“暂无数据”。

已下载并导入 2016–2025 十个完整年度的 NVD Feed：压缩缓存约 152 MB，数据库实测有 251,929 条十年记录，其中 251,922 条带有 NVD 描述（7 条为源数据没有描述的记录）。数据库另有其他年份记录；本次修复后总计 268,983 条。十年完整 CVE 数据不硬编码进 `artex.exe`：这会造成数百 MB 甚至更大的二进制、升级困难和重复占用。更新器和解析器在程序内，Feed 缓存在本地数据目录，索引记录落在 PostgreSQL。当前年度内容通过“更新近 30 天”从 NVD API 补充；如果本机网络无法连接 NVD，可配置 `NVD_API_KEY` 并重试。

修复了导入结构体中 `lang` / `value` JSON tag 冲突的问题，并兼容年度 Feed 不带时区的时间戳。旧库原始 JSON 也缺失描述字段，无法直接回填，因此从本地十个官方年度缓存重新导入；核验 `CVE-2021-44228` 已有完整描述（713 字符）、CVSS 10.0 / CRITICAL、CPE 受影响配置、CWE 与参考信息。`cve_search` 只返回轻量摘要，避免把大量原始 NVD JSON 塞进 Agent 上下文；需要完整条目时由 `cve_get` 加载。

`cve_search` 与 `cve_get` 已绑定到 `mainagent`、`planner`、`worker`、`auto`、`pentest`、`librarian`、`exploiter` 和 `highrisk_chain`。CVE 页面可进入详情页，展示描述、评分、向量、CWE、受影响配置、参考链接、发布时间、修改时间和可折叠的原始 NVD JSON。Chains 页面从后端读取 Agent 实际注入的同一份九类规则，可展开检查问题与转向建议。

## 蜜罐功能

蜜罐识别使用已有资产中的端口、Banner、页面标题、正文和证书字段，不主动连接目标。命中后保存评分、产品和证据，避免只在 UI 临时计算。签名库支持内嵌默认值和 `data/signatures/honeypot.json` 热更新。

当前已完成 L1 被动静态识别；参考项目设计中的行为探针、登录后检查和 planner 自动降级策略仍属于后续阶段，不能宣称已经全部完成。

## 当前优先级

1. CVE 十年导入改为后台任务并增加进度表，避免 HTTP 请求长时间占用连接。
2. 将蜜罐评分写入资产摘要，并让 planner 在高置信度命中时自动降低交互等级。
3. 补齐隧道 `probe → plan → deploy → health → teardown`，在线 deploy 必须经过审批和沙箱。
4. 将会话工具接入 `agent/toolcatalog.go`，统一经过 guard、活动记录和任务范围检查。
5. 为新增 API 增加 PostgreSQL 集成测试和重启后迁移测试。

## Skill 分配审查（本次新增）

原来的启动同步会把 383 个 skill 广泛授权给 `auto`、`pentest`、`planner`、`worker`。这保证了兼容性，但职责边界过宽：规划器和执行器都能看到全部攻击、逆向、云和移动端内容，专用 Agent 反而没有任何 skill。现在新增了插入式职责分配：`mainagent` 获得编排、范围和证据类 skill；`librarian` 获得证据、报告和规范化类 skill；`exploiter` 获得受控利用、漏洞类别、横向与持久化类 skill；`highrisk_chain` 获得链路、范围、横向、隧道相关编排类 skill。同步只新增缺失授权，保留 UI 中的人工禁用，不会覆盖已有配置。

这是一层兼容迁移，尚未删除旧的广泛授权。后续如果希望严格最小权限，应增加“角色模板 + 明确人工确认”的收敛迁移：先展示差异，再由管理员确认后移除不匹配 skill，不能在启动时静默删除。

## 内网能力真实性审计

当前 `E:\777\artex-ko-main` 的内网功能分级如下：

| 能力 | 当前结论 | 运行态证据 |
| --- | --- | --- |
| 会话/凭据台账 | 可用 | `/api/intranet/sessions`、`/api/intranet/credentials` 有 CRUD；当前数据库为 0 条记录 |
| 隧道表 | 可用 | `/api/tunnels` 可读写；当前有 1 条 `chisel / dry_run` 记录 |
| 隧道 dry-run | 可用 | `/api/tunnels/plan` 生成计划、沙箱要求和审批标记 |
| 内置 TCP 转发 | 已接入 | `POST /api/tunnels/{id}/deploy` 在批准后真实监听本机回环端口，双向转发到目标 `host:port`；`health` 会实际拨测目标，`teardown` 会关闭监听 |
| 真正建立 chisel/suo5 隧道 | 条件可用 | Server 会校验二进制、启动真实子进程、等待回环监听、记录 PID，health 检查进程，teardown 终止进程；未配置二进制或 command 时明确报错 |
| WebShell / 反弹 Shell 执行 | WebShell 已接入 | `session.Registry`、`register_session`、`session_exec/read/write` 已装配；反弹 Shell 需要目标侧兼容协议后登记 |
| 被动内网侦察 | 已接入 | `server/session_recon.go` 和 host tool 执行只读命令包并结构化回写资产/fact；不自动扩展 scope |
| 任务级代理热切换 | 未接入 | 当前只有隧道记录字段，没有 upstream proxy 与任务级健康切换运行链 |

因此，“内网页面能用”现在分为两层：WebShell/session_recon 和内置 TCP 转发已经能走真实运行链；Chisel/suo5 也已具备真实进程生命周期，但必须配置对应二进制、回连参数和回环监听地址，缺配置不会伪装成功。自动 scope 扩展审批和反弹 Shell 专用协议仍未完成。

## 基于 GitHub 参考的优化建议

参考仓库当前 main 提交为 `48adc7f1d8f62cbc47e658c9a2b47d6f63ca6cb1`；本地上游 `azrielnan/artex-ko-main` 当前 main 提交为 `4532d79bc4f934435af378c250b92954532c1067`。参考项目的设计文档把内网拆成“会话一等资源 → 被动侦察 → 隧道探测/部署/健康/拆除 → 范围审批 → 证据回写”几个可测试 seam。当前工程已经有数据库台账和 dry-run seam，下一步应按这个顺序补齐：

1. 先移植会话 Registry、加密 secret 存储和 `register_session/session_test/read/write`，让一个本地模拟 WebShell 从探测到执行完整闭环。
2. 再接 `session_recon`，只允许只读命令包，把 IP、路由、邻居和监听端口通过现有资产写入路径回写，并为每条命令保留 evidence。
3. 再接隧道 Manager 的 `probe → deploy → health → teardown`，部署必须经过 guard/审批、沙箱和 scope 校验；不能把 UI 的 `approved=true` 直接当作进程已经启动。
4. 最后把 tunnel upstream 与 task runtime 绑定，实现任务级代理切换、健康失败回滚和重启恢复。
5. 每一步都增加 httptest/模拟目标和重启后 PostgreSQL 集成测试；没有这些测试，页面状态只能证明台账写入成功，不能证明内网链路真实可用。

本次已完成第 1–3 步的可验证子集：会话 Registry 与工具、只读内网侦察、内置 TCP 转发器，以及 Chisel/suo5 外部进程适配器。外部工具路径和 endpoint 仍由部署配置提供，Server 不会下载或猜测第三方二进制。

## 验证记录

- 前端 `npm run build`：通过。
- `go test ./db -run TestImportNVDJSONV2Shape`：通过，验证官方 NVD 2.0 的数组型 `references`、CWE、CVSS、英文描述和流式解析。
- `go test ./agent -run TestListChainGuidesMatchesInjectedScenarios`：通过，验证九类 UI 规则均来自 Worker 场景源。
- `go test ./server ./honeydetect -run '^$'`：编译通过。
- 蜜罐识别单元测试：通过。
- 服务已重新构建并启动，蜜罐页面返回 HTTP 200。
- CVE 数据核验：十年范围 251,929 条，251,922 条有描述；总库 268,983 条。`CVE-2021-44228` 的描述和完整详情字段已修复。
- 运行态页面 `/function/chains/`、`/function/cves/`、`/function/cves/detail?id=CVE-2021-44228` 与 `/api/health` 返回 HTTP 200。需要登录的 `/api/chains` 未带 token 时返回 401，符合认证要求。
- NVD 在线 API 在本机出现 30 秒连接超时，因此十年基线采用官方年度压缩 Feed；当前年度实时更新仍受网络可达性影响。
- 内置 TCP 转发测试：`go test ./server -run 'TestBuiltinTCPForwardCopiesBytes|TestNormalizeTargetRequiresHostPort'` 通过；服务重启后 `/api/health`、`/api/ready` 和隧道页面返回 HTTP 200。
- 会话/侦察测试：`go test ./server ./session ./recon ./db -run 'Test.*(Tunnel|Session|Recon|Forward)' -count=1` 通过；服务重启后 `/api/ready` 返回 200，内网页面与隧道页面返回 HTTP 200。

