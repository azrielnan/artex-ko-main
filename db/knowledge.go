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

type KnowledgeEntry struct {
	ID           int64           `json:"id"`
	CanonicalKey string          `json:"canonical_key"`
	EntryType    string          `json:"entry_type"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	Normalized   json.RawMessage `json:"normalized"`
	Content      string          `json:"content"`
	SourceRefs   json.RawMessage `json:"source_refs"`
	Tags         []string        `json:"tags"`
	CVEIDs       []string        `json:"cve_ids"`
	DedupeHash   string          `json:"dedupe_hash"`
	Confidence   string          `json:"confidence"`
	Status       string          `json:"status"`
	CreatedBy    string          `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func hashKnowledge(canonical, content string) string {
	h := sha256.Sum256([]byte(canonical + "\x00" + content))
	return hex.EncodeToString(h[:])
}
func scanKnowledge(s interface{ Scan(...any) error }) (*KnowledgeEntry, error) {
	k := &KnowledgeEntry{}
	var n, refs []byte
	err := s.Scan(&k.ID, &k.CanonicalKey, &k.EntryType, &k.Title, &k.Summary, &n, &k.Content, &refs, &k.Tags, &k.CVEIDs, &k.DedupeHash, &k.Confidence, &k.Status, &k.CreatedBy, &k.CreatedAt, &k.UpdatedAt)
	k.Normalized = json.RawMessage(n)
	k.SourceRefs = json.RawMessage(refs)
	return k, err
}
func (d *DB) UpsertKnowledgeEntry(k *KnowledgeEntry) (int64, error) {
	if k == nil {
		return 0, fmt.Errorf("nil knowledge entry")
	}
	if k.CanonicalKey == "" {
		k.CanonicalKey = strings.ToLower(strings.TrimSpace(k.Title))
	}
	if k.DedupeHash == "" {
		k.DedupeHash = hashKnowledge(k.CanonicalKey, k.Content)
	}
	if len(k.Normalized) == 0 {
		k.Normalized = []byte(`{}`)
	}
	if len(k.SourceRefs) == 0 {
		k.SourceRefs = []byte(`[]`)
	}
	var id int64
	err := d.QueryRow(`INSERT INTO knowledge_entries(canonical_key,entry_type,title,summary,normalized,content,source_refs,tags,cve_ids,dedupe_hash,confidence,status,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(canonical_key) DO UPDATE SET entry_type=EXCLUDED.entry_type,title=EXCLUDED.title,summary=EXCLUDED.summary,normalized=EXCLUDED.normalized,content=EXCLUDED.content,source_refs=EXCLUDED.source_refs,tags=EXCLUDED.tags,cve_ids=EXCLUDED.cve_ids,dedupe_hash=EXCLUDED.dedupe_hash,confidence=EXCLUDED.confidence,status=EXCLUDED.status,created_by=EXCLUDED.created_by,updated_at=now() RETURNING id`, k.CanonicalKey, k.EntryType, k.Title, k.Summary, k.Normalized, k.Content, k.SourceRefs, k.Tags, k.CVEIDs, k.DedupeHash, k.Confidence, k.Status, k.CreatedBy).Scan(&id)
	return id, err
}
func (d *DB) FindKnowledgeDuplicate(hash string) (*KnowledgeEntry, error) {
	k, err := scanKnowledge(d.QueryRow(`SELECT id,canonical_key,entry_type,title,summary,normalized,content,source_refs,tags,cve_ids,dedupe_hash,confidence,status,created_by,created_at,updated_at FROM knowledge_entries WHERE dedupe_hash=$1 LIMIT 1`, hash))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return k, err
}
func (d *DB) GetKnowledgeEntry(id int64) (*KnowledgeEntry, error) {
	return scanKnowledge(d.QueryRow(`SELECT id,canonical_key,entry_type,title,summary,normalized,content,source_refs,tags,cve_ids,dedupe_hash,confidence,status,created_by,created_at,updated_at FROM knowledge_entries WHERE id=$1`, id))
}
func (d *DB) SearchKnowledgeEntries(query, typ, status string, limit int) ([]*KnowledgeEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{}
	w := []string{"TRUE"}
	if query != "" {
		args = append(args, "%"+query+"%")
		w = append(w, fmt.Sprintf("(title ILIKE $%d OR summary ILIKE $%d OR content ILIKE $%d)", len(args), len(args), len(args)))
	}
	if typ != "" {
		args = append(args, typ)
		w = append(w, fmt.Sprintf("entry_type=$%d", len(args)))
	}
	if status != "" {
		args = append(args, status)
		w = append(w, fmt.Sprintf("status=$%d", len(args)))
	}
	args = append(args, limit)
	rows, err := d.Query(fmt.Sprintf(`SELECT id,canonical_key,entry_type,title,summary,normalized,content,source_refs,tags,cve_ids,dedupe_hash,confidence,status,created_by,created_at,updated_at FROM knowledge_entries WHERE %s ORDER BY updated_at DESC LIMIT $%d`, strings.Join(w, " AND "), len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*KnowledgeEntry
	for rows.Next() {
		k, e := scanKnowledge(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
