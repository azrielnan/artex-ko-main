// Package honeydetect 是批 6 L1 蜜罐静态签名识别层(设计文档
// HONEYPOT-DETECTION-DESIGN.md 二·L1):对 service 资产已知的静态特征
// (banner/HTTP 标题/正文/TLS 证书)做子串签名匹配,产出 0-1 的蜜罐置信度
// 评分与命中证据。签名库默认内嵌在本包(go:embed signatures.json);server 启动
// 时若 dataDir/signatures/honeypot.json 存在则优先用之并按 mtime 热更新(便于
// 不落盘发版地运营签名库),不存在则用内嵌库,永不空载。
// 签名只收录有公开来源的(来源写在每条签名的 source 字段),严禁编造。
package honeydetect

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// embeddedSignatures 是内嵌签名库(与 signatures.json 同文件),作为签名文件
// 缺失时的兜底——平台任何部署形态下签名识别都可用,不依赖 dataDir 种子拷贝。
//
//go:embed signatures.json
var embeddedSignatures []byte

// ServiceView 是一次签名评估的输入视图:调用方填手头已有的字段,没有就留空。
type ServiceView struct {
	Port        int    // 服务端口(0=未知)
	Banner      string // 协议 banner / HTTP Server 头等
	Title       string // HTTP <title>
	Body        string // HTTP 正文(或其片段)
	CertSubject string // TLS 证书 subject
	CertIssuer  string // TLS 证书 issuer
}

// MatchRule 是一条签名的匹配条件:列出的条件【全部】命中才算命中;
// 一个条件都没有的签名永不命中(防"无条件签名匹配一切")。
type MatchRule struct {
	Port              *int   `json:"port,omitempty"`
	BannerSubstr      string `json:"banner_substr,omitempty"`
	HTTPTitleSubstr   string `json:"http_title_substr,omitempty"`
	HTTPBodySubstr    string `json:"http_body_substr,omitempty"`
	CertSubjectSubstr string `json:"cert_subject_substr,omitempty"`
	CertIssuerSubstr  string `json:"cert_issuer_substr,omitempty"`
}

// Signature 是一条蜜罐静态签名。
type Signature struct {
	Name       string    `json:"name"`       // 签名标识(snake_case)
	Product    string    `json:"product"`    // 蜜罐产品名(Cowrie/OpenCanary/...)
	Confidence float64   `json:"confidence"` // 0-1,命中时贡献的评分
	Source     string    `json:"source"`     // 来源(必填,签名只收录有出处的)
	Match      MatchRule `json:"match"`
}

func (m MatchRule) empty() bool {
	return m.Port == nil && m.BannerSubstr == "" && m.HTTPTitleSubstr == "" &&
		m.HTTPBodySubstr == "" && m.CertSubjectSubstr == "" && m.CertIssuerSubstr == ""
}

// matches 报告视图 v 是否命中本条规则(全部列出的条件都命中)。子串匹配大小写
// 不敏感(banner/title/证书书写大小写随实现飘移,签名只取稳定子串)。
func (m MatchRule) matches(v ServiceView) bool {
	if m.empty() {
		return false
	}
	if m.Port != nil && *m.Port != v.Port {
		return false
	}
	if m.BannerSubstr != "" && !containsFold(v.Banner, m.BannerSubstr) {
		return false
	}
	if m.HTTPTitleSubstr != "" && !containsFold(v.Title, m.HTTPTitleSubstr) {
		return false
	}
	if m.HTTPBodySubstr != "" && !containsFold(v.Body, m.HTTPBodySubstr) {
		return false
	}
	if m.CertSubjectSubstr != "" && !containsFold(v.CertSubject, m.CertSubjectSubstr) {
		return false
	}
	if m.CertIssuerSubstr != "" && !containsFold(v.CertIssuer, m.CertIssuerSubstr) {
		return false
	}
	return true
}

func containsFold(haystack, needle string) bool {
	return haystack != "" && strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// Engine 是签名引擎:持有签名文件路径,按 mtime 缓存重载(热更新)。
type Engine struct {
	mu      sync.RWMutex
	path    string
	modTime time.Time
	sigs    []Signature
}

// NewEngine 以指定签名文件路径建引擎。路径缺失/损坏不报错,Evaluate 返回 0 分。
func NewEngine(path string) *Engine { return &Engine{path: path} }

// signatures 返回当前生效签名:签名文件存在时按 mtime 热更新;文件缺失/读不到/
// 解析失败时回落内嵌签名库(只解析一次,后续命中缓存)。
func (e *Engine) signatures() []Signature {
	fi, err := os.Stat(e.path)
	if err != nil {
		e.mu.Lock()
		if e.sigs == nil {
			e.sigs = parseSignatures(embeddedSignatures)
		}
		e.modTime = time.Time{}
		sigs := e.sigs
		e.mu.Unlock()
		return sigs
	}
	e.mu.RLock()
	if !e.needsReload(fi.ModTime()) {
		sigs := e.sigs
		e.mu.RUnlock()
		return sigs
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	sigs := parseSignatures(mustRead(e.path))
	if sigs == nil {
		sigs = parseSignatures(embeddedSignatures)
	}
	e.sigs, e.modTime = sigs, fi.ModTime()
	return e.sigs
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// parseSignatures 解析签名库 JSON;解析失败返回 nil(调用方自行回落)。
func parseSignatures(data []byte) []Signature {
	if len(data) == 0 {
		return nil
	}
	var file struct {
		Signatures []Signature `json:"signatures"`
	}
	if json.Unmarshal(data, &file) != nil {
		return nil
	}
	return file.Signatures
}

// needsReload 报告缓存的签名是否落后于该 mtime(读锁内快速路径)。
func (e *Engine) needsReload(mt time.Time) bool { return !mt.Equal(e.modTime) }

// Evaluate 对视图 v 跑全部签名,返回命中签名中的最高置信度与命中证据
// (每条 "name(product, confidence)");无命中返回 0 分、空证据。
func (e *Engine) Evaluate(v ServiceView) (score float64, evidence []string) {
	for _, s := range e.signatures() {
		if s.Confidence <= 0 || !s.Match.matches(v) {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("%s(%s, %.2f)", s.Name, s.Product, s.Confidence))
		if s.Confidence > score {
			score = s.Confidence
		}
	}
	return score, evidence
}

// defaultSignaturesPath 是默认签名文件相对路径(开发态从仓库根运行即生效);
// server 启动时用 dataDir 绝对路径经 SetSignaturesPath 覆盖。
const defaultSignaturesPath = "data/signatures/honeypot.json"

var defaultEngine = NewEngine(defaultSignaturesPath)

// SetSignaturesPath 覆盖默认引擎的签名文件路径并清空缓存(server 启动时装配)。
func SetSignaturesPath(path string) {
	defaultEngine.mu.Lock()
	defaultEngine.path = path
	defaultEngine.sigs = nil
	defaultEngine.modTime = time.Time{}
	defaultEngine.mu.Unlock()
}

// Evaluate 用默认引擎评估(见 Engine.Evaluate)。
func Evaluate(v ServiceView) (score float64, evidence []string) {
	return defaultEngine.Evaluate(v)
}
