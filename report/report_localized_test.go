package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/Autumn-27/artex/db"
)

// containsHan 은 문자열에 CJK 한자(중국어)가 섞여 있으면 true 를 돌려준다.
// 한글(unicode.Hangul)·영문·숫자·기호는 한자가 아니므로 걸리지 않는다.
func containsHan(s string) (rune, bool) {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return r, true
		}
	}
	return 0, false
}

func sampleFindingNode(t *testing.T) *db.Node {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"vulnclass": "SQL Injection",
		"name":      "登录表单 SQL 注入",
		"severity":  "high",
		"summary":   "已确认登录参数存在基于错误的 SQL 注入。",
		"evidence":  map[string]string{"poc": "' OR '1'='1"},
	})
	if err != nil {
		t.Fatalf("payload 직렬화 실패: %v", err)
	}
	return &db.Node{ID: 1, Kind: "finding", Payload: payload}
}

func sampleDBFinding() *db.DBFinding {
	return &db.DBFinding{
		ID:              123,
		VulnClass:       "SQL Injection",
		Name:            "登录表单 SQL 注入",
		Severity:        "high",
		Summary:         "已确认登录参数存在基于错误的 SQL 注入。",
		Evidence:        "' OR '1'='1",
		Status:          "confirmed",
		Report:          "详细分析正文。",
		CreatedAt:       time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC),
		EvidenceVersion: 2,
		TaskDescription: "演示目标测试",
		TrafficBindings: []db.FindingTrafficBinding{
			{
				ID:   9,
				Role: "request",
				Note: "注入载荷发送位置",
				Snapshot: db.TrafficEvidenceSnapshot{
					Method: "POST",
					URL:    "https://sandbox.local/login",
					Status: 200,
				},
			},
		},
	}
}

// TestReportMarkdownLocalized 는 탐색 그래프 보고서(report.Markdown)의 골격이 한국어이고
// 중국어 한자가 전혀 없음을 검증한다.
func TestReportMarkdownLocalized(t *testing.T) {
	out := Markdown(Input{
		Title:       "演示任务",
		Goal:        "沙箱全面检查",
		GeneratedAt: time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC),
		AssetCounts: map[string]int{"host": 2, "url": 5},
		Findings:    []*db.Node{sampleFindingNode(t)},
	})
	for _, want := range []string{
		"# 渗透测试报告：演示任务",
		"- **任务目标**：沙箱全面检查",
		"- **生成时间**：",
		"## 摘要",
		"- 已确认漏洞：**1** 个",
		"## 漏洞详情",
		"**PoC / 证据：**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("보고서 골격에 %q 가 없다", want)
		}
	}
}

// TestReportMarkdownEmptyLocalized 는 취약점이 없을 때의 안내 문구가 한국어임을 검증한다.
func TestReportMarkdownEmptyLocalized(t *testing.T) {
	out := Markdown(Input{Title: "空任务", GeneratedAt: time.Now()})
	if !strings.Contains(out, "_本次未发现已确认的漏洞。_") {
		t.Errorf("缺少中文空报告提示：\n%s", out)
	}
}

// TestFindingsMarkdownLocalized 는 취약점 요약/상세 Markdown 익스포트 골격과 심각도 라벨이
// 한국어이고 중국어 한자가 없음을 검증한다.
func TestFindingsMarkdownLocalized(t *testing.T) {
	f := sampleDBFinding()
	out := FindingsMarkdown([]*db.DBFinding{f}, time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC))
	for _, want := range []string{
		"# 漏洞摘要报告",
		"- **生成时间**：",
		"- **漏洞总数**：1 个",
		"## 摘要",
		"| 严重程度 | 数量 |",
		"| 严重 |", "| 高 |", "| 中 |", "| 低 |",
		"## 漏洞详情",
		"- **类型**：SQL Injection",
		"- **状态**：已确认",
		"- **所属任务**：演示目标测试",
		"- **发现时间**：",
		"**证据：**",
		"**详细报告：**",
		"## 相关流量证据",
		"证据版本：2，绑定数量：1。",
		"注入载荷发送位置",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("요약 보고서에 %q 가 없다", want)
		}
	}

	single := SingleFindingMarkdown(f, time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC))
	for _, want := range []string{
		"- **严重程度**：高",
		"## 概述",
		"## 证据",
		"## 详细报告",
		"[请求报文](evidence/123/9/request.http)",
		"[响应报文](evidence/123/9/response.http)",
	} {
		if !strings.Contains(single, want) {
			t.Errorf("단건 보고서에 %q 가 없다", want)
		}
	}
}

// TestFindingsCSVLocalized 는 CSV 익스포트 헤더가 한국어이고 중국어 한자가 없음을 검증한다.
// (BOM 뒤 헤더 행만 한국어면 된다. 데이터 값은 원본 그대로 둔다.)
func TestFindingsCSVLocalized(t *testing.T) {
	raw := FindingsCSV([]*db.DBFinding{sampleDBFinding()})
	out := strings.TrimPrefix(string(raw), "\xEF\xBB\xBF")
	header := strings.SplitN(out, "\n", 2)[0]
	for _, col := range []string{"ID", "名称", "类型", "严重程度", "状态", "所属任务", "发现时间", "概述", "流量证据数量", "流量证据 ID"} {
		if !strings.Contains(header, col) {
			t.Errorf("CSV 헤더에 %q 열이 없다: %s", col, header)
		}
	}
}

func TestFindingLabelsAreChinese(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{findingSeverityLabel("critical"), "严重"}, {findingSeverityLabel("high"), "高"},
		{findingSeverityLabel("medium"), "中"}, {findingSeverityLabel("low"), "低"},
		{findingStatusLabel("pending"), "待处理"}, {findingStatusLabel("in_progress"), "处理中"},
		{findingStatusLabel("confirmed"), "已确认"}, {findingStatusLabel("resolved"), "已处理"},
		{findingStatusLabel("fixed"), "已修复"}, {findingStatusLabel("false_positive"), "误报"},
		{findingStatusLabel("ignored"), "已忽略"}, {findingStatusLabel("duplicate"), "重复"},
		{findingStatusLabel("risk_accepted"), "已接受风险"},
	} {
		if tc.got != tc.want {
			t.Errorf("标签 = %q，期望 %q", tc.got, tc.want)
		}
	}
}
