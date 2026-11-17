package agent

import (
	"fmt"
	"strings"
)

type chainGuide struct {
	tag       string
	terms     []string
	questions []string
	turns     []string
}

// ChainGuide is the UI/API-safe representation of the same scenario guidance
// injected into worker prompts. Returning a copy keeps callers from mutating
// the routing table used by agents.
type ChainGuide struct {
	Tag       string   `json:"tag"`
	Name      string   `json:"name"`
	Summary   string   `json:"summary"`
	Questions []string `json:"questions"`
	Turns     []string `json:"turns"`
}

var chainNames = map[string][2]string{
	"web":     {"Web 应用", "认证、注入、上传、SSRF 和业务逻辑验证"},
	"recon":   {"资产侦察", "资产、端口、子域和服务指纹发现"},
	"app":     {"移动与客户端", "移动端、签名、IPC 和客户端信任边界"},
	"ad":      {"身份与域环境", "Active Directory、Kerberos、LDAP 和联邦身份"},
	"priv":    {"权限提升", "Linux / Windows 权限边界与配置验证"},
	"pivot":   {"内网与隧道", "授权范围内的会话、拓扑和隧道验证"},
	"pwn":     {"二进制利用", "可控崩溃、偏移和控制流验证"},
	"reverse": {"逆向分析", "静态、动态和回归证据链"},
	"meta":    {"拦截与误报", "蜜罐、限流、阻断和传输故障的判断"},
}

// ListChainGuides exposes the exact checklists used by IntentChainBlock.
func ListChainGuides() []ChainGuide {
	guides := make([]ChainGuide, 0, len(chainGuides))
	for _, g := range chainGuides {
		label := chainNames[g.tag]
		guides = append(guides, ChainGuide{Tag: g.tag, Name: label[0], Summary: label[1], Questions: append([]string(nil), g.questions...), Turns: append([]string(nil), g.turns...)})
	}
	return guides
}

var chainGuides = []chainGuide{
	{"web", []string{"web", "http", "api", "sql", "xss", "ssrf", "sqli", "idor", "csrf", "上传", "注入", "接口"}, []string{"输入是否到达了预期解析点？", "是否有正常基线与独立反证？", "权限边界和业务影响是否由响应证明？"}, []string{"若回显不可见，改用状态差异、时间差或带外证据", "若同类响应重复，转查认证、路由、缓存或业务状态"}},
	{"recon", []string{"recon", "scan", "discover", "fingerprint", "资产", "端口", "子域", "指纹", "侦察"}, []string{"当前结论来自真实观测还是仅由指纹推断？", "覆盖是否包含不同协议、端口和入口？"}, []string{"重复同类探测无新信息时转为不同数据源或协议", "先记录证据，再扩展下一批目标"}},
	{"app", []string{"apk", "android", "ios", "mobile", "客户端", "移动端", "签名", "ipc"}, []string{"客户端信任边界在哪里？", "结论能否在代理流量或运行时独立复现？"}, []string{"静态线索不足时转到运行时参数和网络请求", "服务端拒绝时检查本地状态是否真正影响授权"}},
	{"ad", []string{"active directory", "kerberos", "ldap", "域控", "域用户", "票据", "saml", "oauth"}, []string{"凭据是否可用，权限是否被实际验证？", "身份关系和目标范围是否属于当前任务？"}, []string{"单凭凭据文件不宣称接管，先在批准范围内验证最小权限", "认证失败时转查票据类型、时钟和委派关系"}},
	{"priv", []string{"privilege", "privesc", "sudo", "提权", "root", "管理员权限", "权限提升"}, []string{"当前权限和目标权限是否由命令输出确认？", "候选路径是否依赖真实配置而非通用清单？"}, []string{"没有可写/可执行证据时转查服务、定时任务和能力配置", "提权后以独立身份查询复核"}},
	{"pivot", []string{"pivot", "tunnel", "socks", "内网", "隧道", "立足点", "网段", "横向"}, []string{"目标网段是否已明确纳入任务授权范围？", "本地代理成功是否由隧道内目标侧验证？"}, []string{"范围外网段先提交人工审批，不因侦察发现自动扩权", "隧道探活失败按层回滚并检查跳板侧证据"}},
	{"pwn", []string{"buffer overflow", "pwn", "栈溢出", "堆溢出", "格式化字符串", "偏移", "rop"}, []string{"崩溃是否可重复且输入可控？", "偏移和地址是否由本次运行证据确定？"}, []string{"先稳定复现并定位控制数据，再构造受控验证", "无崩溃时转查协议 framing、长度和解析边界"}},
	{"reverse", []string{"reverse", "反编译", "逆向", "二进制", "固件", "函数", "调用链"}, []string{"结论是否绑定到符号、偏移或运行时证据？", "是否存在静态分析无法覆盖的动态分支？"}, []string{"静态路径不明确时用动态观察校验调用链", "对关键结论做独立回归并寻找反例"}},
	{"meta", []string{"blocked", "waf", "honeypot", "蜜罐", "限流", "卡住", "拦截", "误报", "forbidden", "429"}, []string{"这是目标系统响应、平台拦截还是传输故障？", "是否已出现足以确认漏洞的决定性证据？"}, []string{"平台拦截和蜜罐信号不是绕过理由；记录证据并停止高风险动作", "同质响应达到约 20 次且无新信号时切换正交假设或收敛"}},
}

// IntentChainBlock routes a compact scenario-specific checklist from intent text.
// It injects only matched guides, with a strict size cap to keep prompt cost fixed.
func IntentChainBlock(text string) string {
	n := strings.ToLower(text)
	var out strings.Builder
	count := 0
	for _, g := range chainGuides {
		matched := false
		for _, term := range g.terms {
			if strings.Contains(n, strings.ToLower(term)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		fmt.Fprintf(&out, "\n### %s 场景反问\n", g.tag)
		for _, q := range g.questions {
			fmt.Fprintf(&out, "- 反问：%s\n", q)
		}
		for _, turn := range g.turns {
			fmt.Fprintf(&out, "- 转向：%s\n", turn)
		}
		count++
		if count >= 2 {
			break
		}
	}
	if out.Len() == 0 {
		return ""
	}
	return "\n\n【Chains 场景化自检（结束本意图前逐项核对）】\n" + out.String() + "- 判定：没有决定性证据时用 record_fact 标记 inferred/unverified；只有已独立复现的漏洞才调用 report_finding。\n"
}
