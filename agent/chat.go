package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/guard"
	"github.com/Autumn-27/artex/intercept"
	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/llm"
	"github.com/Autumn-27/norma/permission"
	"github.com/Autumn-27/norma/subagent"
	actool "github.com/Autumn-27/norma/tool"
	"github.com/Autumn-27/norma/transcript"
)

// ChatAgent is the generic, task-independent conversational runner behind the chat
// page. It generalizes MainAgent.Chat: any agent (built-in OR a custom one, by
// key) can be chatted with, multi-turn history resumed from the transcript. It is
// a PURE ASSISTANT — base tools are the SDK DefaultTools (Bash/Read/Write/Edit/
// LS/Glob/Grep) plus whatever skills/MCP the key is made visible; NO pentest
// graph/task context is injected (that stays exclusive to MainAgent).
type ChatAgent struct {
	prov           llm.Provider
	model          string
	workDir        string
	tx             *transcript.Store
	window         int
	proxyAddr      string
	proxyCACert    string
	webSearch      WebSearchOpts
	guard          *guard.Guard // optional; nil disables intercept hooks for chat
	nonStreamingFn func() bool  // resolver: use non-streaming (Complete) path? (nil = streaming)
	noaEnabledFn   func() bool  // resolver: use experimental noa compaction? (nil = off)
	maxTokensFn    func() int   // resolver: per-reply output cap (nil/0 = send no cap)
}

func NewChatAgent(prov llm.Provider, model, workDir string, tx *transcript.Store, window int) *ChatAgent {
	return &ChatAgent{prov: prov, model: model, workDir: workDir, tx: tx, window: window}
}

// SetNonStreaming wires a resolver deciding whether chat runs use the
// non-streaming model path (true = non-streaming). nil/unset = streaming.
func (c *ChatAgent) SetNonStreaming(fn func() bool) { c.nonStreamingFn = fn }

func (c *ChatAgent) nonStreaming() bool { return c.nonStreamingFn != nil && c.nonStreamingFn() }

// SetNoaEnabled wires a resolver deciding whether chat runs use the experimental
// noa context-compression mechanism. nil/unset = off (built-in compaction). Read
// per run so the settings toggle takes effect without rebuilding the agent.
func (c *ChatAgent) SetNoaEnabled(fn func() bool) { c.noaEnabledFn = fn }

// SetMaxTokens wires a resolver for the per-reply output cap. nil/unset or 0 =
// send no cap and let the endpoint decide. Read per run, like nonStreaming.
func (c *ChatAgent) SetMaxTokens(fn func() int) { c.maxTokensFn = fn }

func (c *ChatAgent) maxTokens() int {
	if c.maxTokensFn == nil {
		return 0
	}
	return c.maxTokensFn()
}

// SetProxy points the chat agent's WebFetch/Bash at the recording proxy plus the
// CA cert it trusts (empty addr = direct). Kept for parity with the other agents.
func (c *ChatAgent) SetProxy(addr, caCert string) { c.proxyAddr, c.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for the chat agent (off by default).
func (c *ChatAgent) SetWebSearch(o WebSearchOpts) { c.webSearch = o }

// SetGuard attaches a guard (with user-configured intercept rules) to this chat
// agent. Must be called before Chat; safe to call multiple times.
func (c *ChatAgent) SetGuard(g *guard.Guard) { c.guard = g }

// chatWorkDirSpec returns a working-directory notice appended to every chat
// agent's system prompt. Mirrors artifactSpec but without pentest-specific
// wording ("payload", "抓响应体") that would be odd in a general assistant.
func chatWorkDirSpec(workDir string) string {
	return "\n\n**文件输出规约**：需要写文件时，一律写到工作目录 " + workDir + "（这是默认 CWD，相对路径即落在这里，也可用该绝对路径）——不要写 /tmp 或其他绝对路径。"
}

// chatSystem renders the DB-managed prompt body for agentKey. Custom agents have
// no per-key in-code default, so DefaultAssistantPrompt is the render fallback.
func chatSystem(agentKey, dataDir, workDir string) string {
	system := renderSystem(agentKey, DefaultAssistantPrompt, chatVars{DataDir: dataDir, Now: nowStr()})
	if agentKey == "reverse" {
		// Keep the coordination contract effective for existing installations whose
		// editable DB prompt predates the dedicated reverse subagents.
		system += reverseCoordinationBlock()
	}
	return system + chatWorkDirSpec(workDir) + langDirective()
}

func reverseCoordinationBlock() string {
	return `

【逆向 Agent 协调边界（运行时强制）】
本会话必须保持逆向与安全测试上下文隔离。复杂分析按以下顺序协调：
1. 用 Agent 调用 reverse-worker 执行一条明确的只读逆向路线；
2. 用 Agent 调用 reverse-regression 独立复核关键结论，寻找反例或证据缺口；
3. 用 Agent 调用 reverse-supervisor 检查证据、范围隔离、回归结果和遗漏；监督未通过时不得宣称结论已确认。
三个子 Agent 只使用只读本地检查工具和当前逆向会话可见的 REA/MCP 工具，不得使用渗透、资产、漏洞上报或平台管理工具。最终回答必须区分直接观察、推断、未知和回归状态。`
}

// reverseSubagentDefinitions describes the isolated specialist lanes exposed
// only by the reverse-analysis conversation agent. The parent decides when to
// delegate; each child receives a focused prompt and its own sidechain transcript.
func reverseSubagentDefinitions() map[string]subagent.Definition {
	return map[string]subagent.Definition{
		"reverse-worker": {
			Description: "执行一条明确的逆向分析路线，返回带证据的事实、偏移、函数或调用链。",
			Prompt:      []string{`你是逆向分析执行子 Agent。只处理父 Agent 分配的一个明确分析问题，使用可用的 REA MCP 和本地只读工具取得证据。不要做网络渗透、漏洞利用、破解或越权操作，不要修改原始样本。输出：已执行的步骤、关键证据、结论、未解决问题和建议的下一步。所有结论标注直接观察或推断。`},
			// REA MCP tools do not advertise read-only metadata; the explicit tool
			// allowlist below is the boundary, so bypass is needed for MCP calls.
			PermissionMode: permission.ModeBypass,
			MaxTurns:       12,
		},
		"reverse-regression": {
			Description:    "独立回归验证逆向结论，尝试复现行为并指出不一致或证据缺口。",
			Prompt:         []string{`你是逆向分析回归测试子 Agent。独立复核父 Agent 或执行子 Agent 提供的结论，优先使用不同的入口、函数引用、版本对照或最小可复现步骤。只读分析，不修改样本，不连接外部目标。明确写出复现结果、失败原因、证据位置和是否通过回归。不要把推测写成事实。`},
			PermissionMode: permission.ModeBypass,
			MaxTurns:       10,
		},
		"reverse-supervisor": {
			Description:    "监督逆向分析质量，检查证据、范围隔离、回归结果和遗漏的风险。",
			Prompt:         []string{`你是逆向分析监督子 Agent。审查父 Agent 汇总的执行与回归结果，检查证据是否可核验、是否混入安全测试上下文、是否遗漏关键引用或相互矛盾。只提出基于现有材料的整改项和下一步，不自行扩展目标，不修改文件。输出通过项、阻塞项、证据缺口和最终建议。`},
			PermissionMode: permission.ModeBypass,
			MaxTurns:       8,
		},
	}
}

func reverseSubagentToolNames(tools []actool.CoreTool) []string {
	var names []string
	for _, t := range tools {
		name := t.Name()
		lower := strings.ToLower(name)
		// Keep the normal local inspection tools and all REA/MCP tools. The
		// subagents intentionally do not receive platform/pentest domain tools.
		if name == "Read" || name == "LS" || name == "Glob" || name == "Grep" ||
			strings.HasPrefix(lower, "mcp__rea__") {
			names = append(names, name)
		}
	}
	return names
}

// chatVars carries the runtime variables a custom agent's prompt may reference.
// DataDir (server data root) + Now (server wall-clock, refreshed each turn) are the
// universal ones; any other {{.X}} fails to render and falls back to
// DefaultAssistantPrompt.
type chatVars struct{ DataDir, Now string }

// Chat runs ONE turn of a conversation with the agent identified by agentKey,
// resuming prior history keyed by sessionID. maxTurns is the per-turn agent step
// budget (0 = unlimited). maxDuration is the wall-clock run budget per turn
// (0 = unlimited); the timer resets each time Chat is called, so a new user
// message always starts a fresh countdown. webSearch gates network search for
// THIS agent (the global backend/key still come from the chat agent's config,
// but each agent decides on/off). emit receives each execution step (thinking /
// tool_use / tool_result / text / result), tagged with the agent key as the
// worker lane.
func (c *ChatAgent) Chat(ctx context.Context, agentKey, sessionID, message string, maxTurns int, maxDuration time.Duration, webSearch bool, emit func(db.Activity)) (string, error) {
	// gate the global web-search opts by this agent's own flag.
	ws := c.webSearch
	if !webSearch {
		ws.Enabled = false
	}

	// Per-session working directory: <workDir>/sessions/<sessionID>/
	// Isolates file writes across conversations, mirroring how workers use i<intentID>/.
	sessionWorkDir := filepath.Join(c.workDir, "sessions", sessionID)
	_ = os.MkdirAll(sessionWorkDir, 0o755)
	ctx = intercept.WithReviewWorkingDirectory(ctx, sessionWorkDir)

	// Pure assistant: DefaultTools as the base; AugmentTools layers in the key's
	// visible skills/MCP and lets the DB tools table filter/override. DefaultTools
	// have no tools-table rows, so they always pass through.
	base := actool.DefaultTools()
	ctx = WithRunInfo(ctx, RunInfo{SessionID: sessionID})
	tools, def, cleanup := AugmentTools(ctx, agentKey, base)
	defer cleanup()
	if agentKey == "reverse" {
		// The reverse parent is the only chat agent allowed to delegate. Child
		// agents inherit the already-connected REA tools but are restricted to
		// read-only inspection names, keeping pentest/platform tools out.
		parentTools := actool.NewRegistry(tools...)
		childDefs := reverseSubagentDefinitions()
		allowedChildTools := reverseSubagentToolNames(tools)
		for key, child := range childDefs {
			child.Tools = allowedChildTools
			childDefs[key] = child
		}
		tools = append(tools, subagent.NewTool(subagent.Config{
			Provider:    c.prov,
			ParentTools: parentTools,
			Agents:      childDefs,
			MaxDepth:    1,
			Transcript:  c.tx,
		}))
	}

	system, boundary := deferredSystem(chatSystem(agentKey, c.workDir, sessionWorkDir), def)
	opts := agentcore.Options{
		Provider:        c.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // 走记录代理留痕；载入代理 CA 验证 MITM 重签的 HTTPS 证书
		WebFetchProxy:   c.proxyAddr,
		WebFetchCACert:  c.proxyCACert,
		// 联网搜索(可选)。ddgs 无需 key；brave-free 需 BraveKey；tavily 需 TavilyKey。
		// WebSearchProxy 是独立出口代理(http/https/socks5)，与记录流量的 MITM 代理无关；空则直连。
		EnableWebSearch:       ws.Enabled,
		WebSearchBackend:      ws.Backend,
		BraveSearchAPIKey:     ws.BraveKey,
		TavilySearchAPIKey:    ws.TavilyKey,
		DeepSeekSearchBaseURL: ws.DeepSeekBaseURL,
		DeepSeekSearchAPIKey:  ws.DeepSeekAPIKey,
		DeepSeekSearchModel:   ws.DeepSeekModel,
		WebSearchProxy:        ws.Proxy,
		BashEnv:               proxyEnv(c.proxyAddr, c.proxyCACert), // Bash 子命令默认走代理+信任 CA
		WorkingDir:            sessionWorkDir,
		MaxTurns:              maxTurns,
		MaxDuration:           maxDuration,
		Compaction:            compactionConfig(c.window),
		Todos:                 actool.NewTodoStore(),
		// large tool output spills to cmd-output/ under the session dir.
		// 截断上限用 SDK 默认(tool.Capture 的 30000 字符)。
		ToolOutputDir: filepath.Join(sessionWorkDir, "cmd-output"),
		// 命中预算(步数)→ SDK 跑收尾:输出一句总结。Prompt 与收尾轮数按本 agent key 后台可编辑
		// (自定义 agent 各自一份;留空/0 用通用默认:10 轮)。
		Settlement:   wrapupSettlement(agentKey, nil),
		NonStreaming: c.nonStreaming(), // 该 profile 选非流式时走 Provider.Complete
		MaxTokens:    c.maxTokens(),    // 0 = 不发上限,由服务端默认值决定
	}
	if c.guard != nil {
		opts.Hooks = c.guard.Hooks()
	}
	if c.tx != nil { // persist raw human↔AI conversation; one accumulating file per thread
		opts.Transcript = c.tx
		opts.SessionID = sessionID
	}
	// 实验功能:开启后由 noa 接管上下文压缩(归档集中在 <workDir>/noa/<SessionID> 下,持久)。
	enableNoa(&opts, c.noaEnabledFn, c.workDir, "chat-"+sessionID, noaWarn("chat-"+sessionID))
	ctx = attachSideCapture(ctx, &opts)
	s := agentcore.NewSession(opts)
	defer s.Close()
	// reload prior conversation so the agent has context across turns (each Chat is
	// a fresh session). First turn: no file yet → Resume loads nothing and proceeds.
	if c.tx != nil {
		_ = s.Resume(sessionID)
	}
	// re-unlock skill-gated MCPs from prior Skill() calls in the reloaded history so
	// revealed tools stay callable across the fresh session.
	seedUnlockFromHistory(s.Messages(), def.UnlockSkill)
	text, _, err := captureRunSession(ctx, s, message, func(r db.Activity) {
		if emit != nil {
			r.Worker = agentKey
			emit(r)
		}
	})
	return text, err
}
