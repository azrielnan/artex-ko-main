package honeydetect

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSigs 在临时目录落一份签名文件并返回路径。
func writeSigs(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "honeypot.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const testSigs = `{"signatures":[
  {"name":"cowrie_default_ssh_banner","product":"Cowrie","confidence":0.99,"source":"t","match":{"banner_substr":"OpenSSH_6.0p1 Debian-4+deb7u2"}},
  {"name":"hfish_default_web_xjs","product":"HFish","confidence":0.85,"source":"t","match":{"port":8080,"http_body_substr":"x.js"}},
  {"name":"dionaea_default_cert_subject","product":"Dionaea","confidence":0.95,"source":"t","match":{"cert_subject_substr":"Nepenthes Development Team"}},
  {"name":"empty_match_never_hits","product":"X","confidence":1.0,"source":"t","match":{}}
]}`

func TestEvaluateHitsHighestConfidence(t *testing.T) {
	e := NewEngine(writeSigs(t, testSigs))
	score, ev := e.Evaluate(ServiceView{Port: 22, Banner: "SSH-2.0-OpenSSH_6.0p1 Debian-4+deb7u2"})
	if score != 0.99 || len(ev) != 1 {
		t.Fatalf("cowrie banner: score=%v ev=%v", score, ev)
	}
}

func TestEvaluateAllConditionsMustMatch(t *testing.T) {
	e := NewEngine(writeSigs(t, testSigs))
	// 端口不对:body 命中也不算
	if score, _ := e.Evaluate(ServiceView{Port: 80, Body: "see x.js"}); score != 0 {
		t.Fatalf("port mismatch should not hit, score=%v", score)
	}
	// 端口+body 都命中才算
	if score, _ := e.Evaluate(ServiceView{Port: 8080, Body: "see x.js"}); score != 0.85 {
		t.Fatalf("port+body hit: score=%v", score)
	}
}

func TestEvaluateCertAndCaseFold(t *testing.T) {
	e := NewEngine(writeSigs(t, testSigs))
	score, _ := e.Evaluate(ServiceView{CertSubject: "C=DE, CN=nepenthes development team, O=dionaea.carnivore.it"})
	if score != 0.95 {
		t.Fatalf("cert subject case-fold hit: score=%v", score)
	}
}

func TestEvaluateNoMatch(t *testing.T) {
	e := NewEngine(writeSigs(t, testSigs))
	score, ev := e.Evaluate(ServiceView{Port: 22, Banner: "SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.6"})
	if score != 0 || ev != nil {
		t.Fatalf("no match: score=%v ev=%v", score, ev)
	}
}

func TestMissingOrCorruptFileDegradesQuietly(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "nonexistent.json"))
	if score, _ := e.Evaluate(ServiceView{Banner: "OpenSSH_6.0p1 Debian-4+deb7u2"}); score != 0 {
		t.Fatalf("missing file should score 0, got %v", score)
	}
	e2 := NewEngine(writeSigs(t, `{not json`))
	if score, _ := e2.Evaluate(ServiceView{Banner: "OpenSSH_6.0p1 Debian-4+deb7u2"}); score != 0 {
		t.Fatalf("corrupt file should score 0, got %v", score)
	}
}

func TestHotReloadByMtime(t *testing.T) {
	p := writeSigs(t, `{"signatures":[]}`)
	e := NewEngine(p)
	if score, _ := e.Evaluate(ServiceView{Banner: "OpenSSH_6.0p1 Debian-4+deb7u2"}); score != 0 {
		t.Fatalf("empty sigs should score 0, got %v", score)
	}
	// 重写文件(睡眠确保 mtime 变化可观测),无需重启即应热更新。
	if err := os.Chtimes(p, mustStat(t, p).ModTime(), mustStat(t, p).ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(testSigs), 0o644); err != nil {
		t.Fatal(err)
	}
	if score, _ := e.Evaluate(ServiceView{Banner: "OpenSSH_6.0p1 Debian-4+deb7u2"}); score != 0.99 {
		t.Fatalf("hot reload should pick up new sigs, score=%v", score)
	}
}

func mustStat(t *testing.T, p string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

// TestBundledSignaturesLoad 仓库内嵌签名库必须可解析、且每条都有来源与条件。
func TestBundledSignaturesLoad(t *testing.T) {
	sigs := parseSignatures(embeddedSignatures)
	if len(sigs) == 0 {
		t.Fatal("内嵌签名库为空或解析失败")
	}
	for _, s := range sigs {
		if s.Name == "" || s.Product == "" || s.Source == "" {
			t.Fatalf("签名缺 name/product/source: %+v", s)
		}
		if s.Confidence < 0.85 || s.Confidence > 0.99 {
			t.Fatalf("签名 %s 置信度 %.2f 不在 0.85-0.99 区间", s.Name, s.Confidence)
		}
		if s.Match.empty() {
			t.Fatalf("签名 %s 没有任何匹配条件", s.Name)
		}
	}
	// 内嵌库在签名文件缺失时应作为兜底返回(抽两条做实命中验证)。
	e := NewEngine(filepath.Join(t.TempDir(), "nonexistent.json"))
	score, _ := e.Evaluate(ServiceView{Port: 22, Banner: "SSH-2.0-OpenSSH_6.0p1 Debian-4+deb7u2"})
	if score != 0.99 {
		t.Fatalf("内嵌 Cowrie 签名未命中, score=%v", score)
	}
	score, _ = e.Evaluate(ServiceView{CertIssuer: "C=DE, CN=Nepenthes Development Team, O=dionaea.carnivore.it, OU=anv"})
	if score != 0.9 {
		t.Fatalf("内置 Dionaea issuer 签名未命中, score=%v", score)
	}
}
