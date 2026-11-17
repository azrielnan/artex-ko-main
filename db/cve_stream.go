package db

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type nvdStreamVulnerability struct {
	CVE struct {
		ID           string `json:"id"`
		Descriptions []struct {
			Lang  string `json:"lang"`
			Value string `json:"value"`
		} `json:"descriptions"`
		Published      json.RawMessage            `json:"published"`
		LastModified   json.RawMessage            `json:"lastModified"`
		Metrics        map[string]json.RawMessage `json:"metrics"`
		Configurations json.RawMessage            `json:"configurations"`
		Weaknesses     []struct {
			Description []struct {
				Value string `json:"value"`
			} `json:"description"`
		} `json:"weaknesses"`
		References []struct {
			URL string `json:"url"`
		} `json:"references"`
	} `json:"cve"`
}

// ImportNVDReader reads the large NVD 2.0 feed incrementally and commits once per feed.
func (d *DB) ImportNVDReader(r io.Reader) (int, error) {
	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return 0, err
	}
	if tok != json.Delim('{') {
		return 0, fmt.Errorf("NVD JSON 根节点必须是对象")
	}
	tx, err := d.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	count, found := 0, false
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return count, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return count, fmt.Errorf("NVD JSON 字段名无效")
		}
		if key != "vulnerabilities" {
			var discard json.RawMessage
			if err := dec.Decode(&discard); err != nil {
				return count, err
			}
			continue
		}
		found = true
		arr, err := dec.Token()
		if err != nil {
			return count, err
		}
		if arr != json.Delim('[') {
			return count, fmt.Errorf("NVD vulnerabilities 必须是数组")
		}
		for dec.More() {
			var item nvdStreamVulnerability
			if err := dec.Decode(&item); err != nil {
				return count, err
			}
			if err := upsertNVDStreamRecord(tx, &item); err != nil {
				return count, err
			}
			count++
		}
		if _, err := dec.Token(); err != nil {
			return count, err
		}
	}
	if _, err := dec.Token(); err != nil {
		return count, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return count, fmt.Errorf("NVD JSON 根节点后存在额外内容")
		}
		return count, err
	}
	if !found {
		return count, fmt.Errorf("NVD JSON 缺少 vulnerabilities 数组")
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}

func upsertNVDStreamRecord(q cveQueryer, v *nvdStreamVulnerability) error {
	if strings.TrimSpace(v.CVE.ID) == "" {
		return nil
	}
	description := ""
	for _, item := range v.CVE.Descriptions {
		if item.Lang == "en" {
			description = item.Value
			break
		}
		if description == "" {
			description = item.Value
		}
	}
	var cwes []string
	for _, weakness := range v.CVE.Weaknesses {
		for _, item := range weakness.Description {
			cwes = append(cwes, item.Value)
		}
	}
	refs := make([]string, 0, len(v.CVE.References))
	for _, ref := range v.CVE.References {
		if ref.URL != "" {
			refs = append(refs, ref.URL)
		}
	}
	references, _ := json.Marshal(refs)
	raw, _ := json.Marshal(v.CVE)
	if cwes == nil {
		cwes = []string{}
	}
	c := &CVERecord{CVEID: v.CVE.ID, Source: "nvd", Description: description, PublishedAt: parseNVDTime(v.CVE.Published), ModifiedAt: parseNVDTime(v.CVE.LastModified), CWEs: cwes, References: references, Affected: v.CVE.Configurations, Raw: raw}
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
	_, err := upsertCVE(q, c)
	return err
}
