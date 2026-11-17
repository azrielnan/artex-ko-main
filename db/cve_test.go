package db

import (
	"bytes"
	"testing"
)

func TestImportNVDJSONV2Shape(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer d.Close()
	const id = "CVE-2099-999999"
	defer d.Exec(`DELETE FROM cve_records WHERE cve_id=$1`, id)
	body := []byte(`{"vulnerabilities":[{"cve":{"id":"CVE-2099-999999","sourceIdentifier":"nvd@example.test","published":"2026-01-01T00:00:00.000","lastModified":"2026-01-02T00:00:00.000","descriptions":[{"lang":"es","value":"Descripción"},{"lang":"en","value":"English description"}],"metrics":{"cvssMetricV31":[{"cvssData":{"baseScore":9.8,"vectorString":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H","baseSeverity":"CRITICAL"}}]},"weaknesses":[{"description":[{"value":"CWE-502"}]}],"configurations":[],"references":[{"url":"https://example.test/advisory","tags":["Vendor Advisory"]}]}}]}`)
	n, err := d.ImportNVDReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("official NVD 2.0 shape failed to import: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported=%d, want 1", n)
	}
	c, err := d.GetCVE(id)
	if err != nil {
		t.Fatal(err)
	}
	searched, err := d.SearchCVEs(id, "", 5)
	if err != nil || len(searched) == 0 {
		t.Fatalf("search should return imported CVE, rows=%d err=%v", len(searched), err)
	}
	if c.Description != "English description" || c.CVSSScore == nil || *c.CVSSScore != 9.8 || len(c.CWEs) != 1 || string(c.References) != `["https://example.test/advisory"]` {
		t.Fatalf("parsed fields are incomplete: %+v refs=%s", c, c.References)
	}
}
