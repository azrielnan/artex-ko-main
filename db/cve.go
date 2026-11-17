package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CVERecord struct {
	ID          int64           `json:"id"`
	CVEID       string          `json:"cve_id"`
	Source      string          `json:"source"`
	Description string          `json:"description"`
	PublishedAt *time.Time      `json:"published_at"`
	ModifiedAt  *time.Time      `json:"modified_at"`
	CVSSScore   *float64        `json:"cvss_score"`
	CVSSVector  string          `json:"cvss_vector"`
	Severity    string          `json:"severity"`
	CWEs        []string        `json:"cwes"`
	References  json.RawMessage `json:"references"`
	Affected    json.RawMessage `json:"affected"`
	Raw         json.RawMessage `json:"raw"`
	DedupeHash  string          `json:"dedupe_hash"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func parseNVDTime(raw json.RawMessage) *time.Time {
	var value string
	if json.Unmarshal(raw, &value) != nil || value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.000", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return &t
		}
	}
	return nil
}

func scanCVE(s interface{ Scan(...any) error }) (*CVERecord, error) {
	c := &CVERecord{}
	var refs, aff, raw []byte
	var cweText string
	err := s.Scan(&c.ID, &c.CVEID, &c.Source, &c.Description, &c.PublishedAt, &c.ModifiedAt, &c.CVSSScore, &c.CVSSVector, &c.Severity, &cweText, &refs, &aff, &raw, &c.DedupeHash, &c.CreatedAt, &c.UpdatedAt)
	c.CWEs = parsePGTextArray(cweText)
	c.References = json.RawMessage(refs)
	c.Affected = json.RawMessage(aff)
	c.Raw = json.RawMessage(raw)
	return c, err
}

func parsePGTextArray(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "{}" {
		return []string{}
	}
	value = strings.TrimPrefix(strings.TrimSuffix(value, "}"), "{")
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.Trim(parts[i], `" `)
	}
	return parts
}
func cveHash(cve string, raw []byte) string {
	h := sha256.Sum256(append([]byte(cve+"\x00"), raw...))
	return hex.EncodeToString(h[:])
}

type cveQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func upsertCVE(q cveQueryer, c *CVERecord) (int64, error) {
	if c == nil || strings.TrimSpace(c.CVEID) == "" {
		return 0, fmt.Errorf("cve_id required")
	}
	if len(c.Raw) == 0 {
		c.Raw = []byte(`{}`)
	}
	if len(c.References) == 0 {
		c.References = []byte(`[]`)
	}
	if len(c.Affected) == 0 {
		c.Affected = []byte(`{}`)
	}
	if c.CWEs == nil {
		c.CWEs = []string{}
	}
	if c.DedupeHash == "" {
		c.DedupeHash = cveHash(c.CVEID, c.Raw)
	}
	var id int64
	err := q.QueryRow(`INSERT INTO cve_records(cve_id,source,description,published_at,modified_at,cvss_score,cvss_vector,severity,cwes,"references",affected,raw,dedupe_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(cve_id) DO UPDATE SET source=EXCLUDED.source,description=EXCLUDED.description,published_at=EXCLUDED.published_at,modified_at=EXCLUDED.modified_at,cvss_score=EXCLUDED.cvss_score,cvss_vector=EXCLUDED.cvss_vector,severity=EXCLUDED.severity,cwes=EXCLUDED.cwes,"references"=EXCLUDED."references",affected=EXCLUDED.affected,raw=EXCLUDED.raw,dedupe_hash=EXCLUDED.dedupe_hash,updated_at=now() RETURNING id`, c.CVEID, c.Source, c.Description, c.PublishedAt, c.ModifiedAt, c.CVSSScore, c.CVSSVector, c.Severity, c.CWEs, c.References, c.Affected, c.Raw, c.DedupeHash).Scan(&id)
	return id, err
}
func (d *DB) UpsertCVE(c *CVERecord) (int64, error) { return upsertCVE(d, c) }
func (d *DB) GetCVE(id string) (*CVERecord, error) {
	c, e := scanCVE(d.QueryRow(`SELECT id,cve_id,source,description,published_at,modified_at,cvss_score,cvss_vector,severity,cwes,"references",affected,raw,dedupe_hash,created_at,updated_at FROM cve_records WHERE cve_id=$1`, id))
	return c, e
}
func (d *DB) SearchCVEs(q, severity string, limit int) ([]*CVERecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{}
	w := []string{"TRUE"}
	if q != "" {
		args = append(args, "%"+q+"%")
		likeArg := len(args)
		args = append(args, q)
		ftsArg := len(args)
		w = append(w, fmt.Sprintf("(cve_id ILIKE $%d OR to_tsvector('simple', cve_id || ' ' || description) @@ plainto_tsquery('simple', $%d))", likeArg, ftsArg))
	}
	if severity != "" {
		args = append(args, severity)
		w = append(w, fmt.Sprintf("severity=$%d", len(args)))
	}
	args = append(args, limit)
	rows, e := d.Query(fmt.Sprintf(`SELECT id,cve_id,source,description,published_at,modified_at,cvss_score,cvss_vector,severity,cwes,"references",affected,raw,dedupe_hash,created_at,updated_at FROM cve_records WHERE %s ORDER BY modified_at DESC NULLS LAST LIMIT $%d`, strings.Join(w, " AND "), len(args)), args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []*CVERecord
	for rows.Next() {
		c, e := scanCVE(rows)
		if e != nil {
			return nil, e
		}
		// Search is the compact discovery surface (including for LLM tools).
		// Full raw NVD JSON and CPE configurations are available through GetCVE;
		// returning them for every search hit can consume the agent context window.
		c.Raw = nil
		c.Affected = nil
		c.References = nil
		out = append(out, c)
	}
	return out, rows.Err()
}
func (d *DB) ImportNVDJSON(data []byte) (int, error) {
	var doc struct {
		Vulnerabilities []struct {
			CVE struct {
				ID           string `json:"id"`
				Descriptions []struct {
					Lang  string `json:"lang"`
					Value string `json:"value"`
				} `json:"descriptions"`
				Published, LastModified json.RawMessage            `json:"published"`
				Metrics                 map[string]json.RawMessage `json:"metrics"`
				Configurations          json.RawMessage            `json:"configurations"`
				Weaknesses              []struct {
					Description []struct {
						Value string `json:"value"`
					} `json:"description"`
				} `json:"weaknesses"`
				References []struct {
					URL string `json:"url"`
				} `json:"references"`
			} `json:"cve"`
		} `json:"vulnerabilities"`
	}
	if e := json.Unmarshal(data, &doc); e != nil {
		return 0, e
	}
	n := 0
	tx, txErr := d.Begin()
	if txErr != nil {
		return 0, txErr
	}
	defer tx.Rollback()
	for _, v := range doc.Vulnerabilities {
		desc := ""
		for _, d := range v.CVE.Descriptions {
			if d.Lang == "en" {
				desc = d.Value
				break
			}
			if desc == "" {
				desc = d.Value
			}
		}
		var cwes []string
		for _, w := range v.CVE.Weaknesses {
			for _, d := range w.Description {
				cwes = append(cwes, d.Value)
			}
		}
		refs := []string{}
		for _, r := range v.CVE.References {
			refs = append(refs, r.URL)
		}
		rb, _ := json.Marshal(refs)
		var raw json.RawMessage
		raw, _ = json.Marshal(v.CVE)
		c := &CVERecord{CVEID: v.CVE.ID, Source: "nvd", Description: desc, PublishedAt: parseNVDTime(v.CVE.Published), ModifiedAt: parseNVDTime(v.CVE.LastModified), CWEs: cwes, References: rb, Affected: v.CVE.Configurations, Raw: raw}
		for _, key := range []string{"cvssMetricV31", "cvssMetricV30", "cvssMetricV2"} {
			var metrics []struct {
				CVSSData struct {
					BaseScore    *float64 `json:"baseScore"`
					Vector       string   `json:"vectorString"`
					BaseSeverity string   `json:"baseSeverity"`
				} `json:"cvssData"`
				BaseSeverity string `json:"baseSeverity"`
			}
			if json.Unmarshal(v.CVE.Metrics[key], &metrics) == nil && len(metrics) > 0 {
				c.CVSSScore = metrics[0].CVSSData.BaseScore
				c.CVSSVector = metrics[0].CVSSData.Vector
				c.Severity = metrics[0].CVSSData.BaseSeverity
				if c.Severity == "" {
					c.Severity = metrics[0].BaseSeverity
				}
				break
			}
		}
		if _, e := upsertCVE(tx, c); e != nil {
			return n, e
		}
		n++
	}
	if e := tx.Commit(); e != nil {
		return n, e
	}
	return n, nil
}
