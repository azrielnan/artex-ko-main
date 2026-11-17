package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Autumn-27/artex/honeydetect"
)

type HoneypotAssessment struct {
	ID        int64           `json:"id"`
	AssetID   int64           `json:"asset_id"`
	TaskID    int64           `json:"task_id"`
	Score     float64         `json:"score"`
	Products  []string        `json:"products"`
	Evidence  json.RawMessage `json:"evidence"`
	ScannedAt time.Time       `json:"scanned_at"`
}

func (d *DB) UpsertHoneypotAssessment(a *HoneypotAssessment) error {
	if a == nil || a.AssetID <= 0 {
		return fmt.Errorf("asset_id required")
	}
	if len(a.Evidence) == 0 {
		a.Evidence = []byte(`[]`)
	}
	_, err := d.Exec(`INSERT INTO honeypot_assessments(asset_id,task_id,score,products,evidence,scanned_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(asset_id,task_id) DO UPDATE SET score=EXCLUDED.score,products=EXCLUDED.products,evidence=EXCLUDED.evidence,scanned_at=now()`, a.AssetID, a.TaskID, a.Score, a.Products, a.Evidence)
	return err
}

func (d *DB) ListHoneypotAssessments(taskID int64, minScore float64) ([]*HoneypotAssessment, error) {
	q := `SELECT id,asset_id,task_id,score,products,evidence,scanned_at FROM honeypot_assessments WHERE task_id=$1 AND score >= $2 ORDER BY score DESC, scanned_at DESC LIMIT 1000`
	rows, err := d.Query(q, taskID, minScore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*HoneypotAssessment
	for rows.Next() {
		a := &HoneypotAssessment{}
		var ev []byte
		if err := rows.Scan(&a.ID, &a.AssetID, &a.TaskID, &a.Score, &a.Products, &ev, &a.ScannedAt); err != nil {
			return nil, err
		}
		a.Evidence = ev
		out = append(out, a)
	}
	return out, rows.Err()
}

func honeypotView(a *Asset) honeydetect.ServiceView {
	v := honeydetect.ServiceView{Port: 0, Title: a.PageTitle, Banner: a.ServiceName}
	if a.Port != nil {
		v.Port = *a.Port
	}
	for k, dst := range map[string]*string{"banner": &v.Banner, "http_title": &v.Title, "http_body": &v.Body, "body": &v.Body, "cert_subject": &v.CertSubject, "cert_issuer": &v.CertIssuer} {
		if x, ok := a.Extra[k].(string); ok && strings.TrimSpace(x) != "" {
			*dst = x
		}
	}
	return v
}

func EvaluateAssetHoneypot(a *Asset) (float64, []string) {
	if a == nil {
		return 0, nil
	}
	return honeydetect.Evaluate(honeypotView(a))
}
