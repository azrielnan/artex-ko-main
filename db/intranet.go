package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type IntranetSession struct {
	ID            int64           `json:"id"`
	TaskID        *int64          `json:"task_id"`
	Name          string          `json:"name"`
	Kind          string          `json:"kind"`
	Target        string          `json:"target"`
	Language      string          `json:"language"`
	Status        string          `json:"status"`
	Endpoint      string          `json:"endpoint"`
	CredentialRef string          `json:"credential_ref"`
	Evidence      json.RawMessage `json:"evidence"`
	CreatedBy     string          `json:"created_by"`
	LastSeen      *time.Time      `json:"last_seen,omitempty"`
	CreatedAt     *time.Time      `json:"created_at,omitempty"`
	UpdatedAt     *time.Time      `json:"updated_at,omitempty"`
}
type Credential struct {
	ID              int64           `json:"id"`
	CredentialRef   string          `json:"credential_ref"`
	Kind            string          `json:"kind"`
	SourceSessionID *int64          `json:"source_session_id"`
	SourceHost      string          `json:"source_host"`
	Scope           string          `json:"scope"`
	Status          string          `json:"status"`
	SecretHint      string          `json:"secret_hint,omitempty"`
	Evidence        json.RawMessage `json:"evidence"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (d *DB) ListIntranetSessions(taskID int64) ([]*IntranetSession, error) {
	q := `SELECT id,task_id,name,kind,target,language,status,endpoint,credential_ref,evidence,created_by,last_seen,created_at,updated_at FROM intranet_sessions`
	var args []any
	if taskID > 0 {
		q += ` WHERE task_id=$1`
		args = append(args, taskID)
	}
	q += ` ORDER BY updated_at DESC LIMIT 500`
	rows, e := d.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []*IntranetSession
	for rows.Next() {
		x := &IntranetSession{}
		var ev []byte
		if e := rows.Scan(&x.ID, &x.TaskID, &x.Name, &x.Kind, &x.Target, &x.Language, &x.Status, &x.Endpoint, &x.CredentialRef, &ev, &x.CreatedBy, &x.LastSeen, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		x.Evidence = ev
		out = append(out, x)
	}
	return out, rows.Err()
}
func (d *DB) CreateIntranetSession(x *IntranetSession) (int64, error) {
	if x == nil || strings.TrimSpace(x.Name) == "" {
		return 0, fmt.Errorf("name required")
	}
	if x.Status == "" {
		x.Status = "planned"
	}
	if len(x.Evidence) == 0 {
		x.Evidence = []byte(`{}`)
	}
	var id int64
	e := d.QueryRow(`INSERT INTO intranet_sessions(task_id,name,kind,target,language,status,endpoint,credential_ref,evidence,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, x.TaskID, x.Name, x.Kind, x.Target, x.Language, x.Status, x.Endpoint, x.CredentialRef, x.Evidence, x.CreatedBy).Scan(&id)
	return id, e
}
func (d *DB) UpdateIntranetSessionStatus(id int64, status string) error {
	_, e := d.Exec(`UPDATE intranet_sessions SET status=$2,last_seen=CASE WHEN $2='active' THEN now() ELSE last_seen END WHERE id=$1`, id, status)
	return e
}
func (d *DB) ListCredentials(status string) ([]*Credential, error) {
	q := `SELECT id,credential_ref,kind,source_session_id,source_host,scope,status,secret_hint,evidence,created_by,created_at,updated_at FROM credentials`
	var args []any
	if status != "" {
		q += ` WHERE status=$1`
		args = append(args, status)
	}
	q += ` ORDER BY updated_at DESC LIMIT 500`
	rows, e := d.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []*Credential
	for rows.Next() {
		x := &Credential{}
		var ev []byte
		if e := rows.Scan(&x.ID, &x.CredentialRef, &x.Kind, &x.SourceSessionID, &x.SourceHost, &x.Scope, &x.Status, &x.SecretHint, &ev, &x.CreatedBy, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		x.Evidence = ev
		out = append(out, x)
	}
	return out, rows.Err()
}
func (d *DB) UpsertCredential(x *Credential) (int64, error) {
	if x == nil || strings.TrimSpace(x.CredentialRef) == "" {
		return 0, fmt.Errorf("credential_ref required")
	}
	if len(x.Evidence) == 0 {
		x.Evidence = []byte(`{}`)
	}
	var id int64
	e := d.QueryRow(`INSERT INTO credentials(credential_ref,kind,source_session_id,source_host,scope,status,secret_hint,evidence,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(credential_ref) DO UPDATE SET kind=EXCLUDED.kind,source_session_id=EXCLUDED.source_session_id,source_host=EXCLUDED.source_host,scope=EXCLUDED.scope,status=EXCLUDED.status,secret_hint=EXCLUDED.secret_hint,evidence=EXCLUDED.evidence,updated_at=now() RETURNING id`, x.CredentialRef, x.Kind, x.SourceSessionID, x.SourceHost, x.Scope, x.Status, x.SecretHint, x.Evidence, x.CreatedBy).Scan(&id)
	return id, e
}
