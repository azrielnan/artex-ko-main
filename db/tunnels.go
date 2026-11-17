package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Tunnel struct {
	ID              int64           `json:"id"`
	TaskID          *int64          `json:"task_id"`
	FindingID       *int64          `json:"finding_id"`
	Name            string          `json:"name"`
	Kind            string          `json:"kind"`
	Status          string          `json:"status"`
	Target          string          `json:"target"`
	ListenAddr      string          `json:"listen_addr"`
	RemoteAddr      string          `json:"remote_addr"`
	CredentialsRef  string          `json:"credentials_ref"`
	Command         string          `json:"command"`
	DryRunScript    string          `json:"dry_run_script"`
	Evidence        json.RawMessage `json:"evidence"`
	SandboxRequired bool            `json:"sandbox_required"`
	Approved        bool            `json:"approved"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func scanTunnel(s interface{ Scan(...any) error }) (*Tunnel, error) {
	t := &Tunnel{}
	var ev []byte
	err := s.Scan(&t.ID, &t.TaskID, &t.FindingID, &t.Name, &t.Kind, &t.Status, &t.Target, &t.ListenAddr, &t.RemoteAddr, &t.CredentialsRef, &t.Command, &t.DryRunScript, &ev, &t.SandboxRequired, &t.Approved, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	t.Evidence = json.RawMessage(ev)
	return t, err
}
func (d *DB) CreateTunnel(t *Tunnel) (int64, error) {
	if t == nil || t.Name == "" || t.Kind == "" {
		return 0, fmt.Errorf("name and kind required")
	}
	if len(t.Evidence) == 0 {
		t.Evidence = []byte(`{}`)
	}
	var id int64
	e := d.QueryRow(`INSERT INTO tunnels(task_id,finding_id,name,kind,status,target,listen_addr,remote_addr,credentials_ref,command,dry_run_script,evidence,sandbox_required,approved,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`, t.TaskID, t.FindingID, t.Name, t.Kind, t.Status, t.Target, t.ListenAddr, t.RemoteAddr, t.CredentialsRef, t.Command, t.DryRunScript, t.Evidence, t.SandboxRequired, t.Approved, t.CreatedBy).Scan(&id)
	return id, e
}
func (d *DB) GetTunnel(id int64) (*Tunnel, error) {
	t, e := scanTunnel(d.QueryRow(`SELECT id,task_id,finding_id,name,kind,status,target,listen_addr,remote_addr,credentials_ref,command,dry_run_script,evidence,sandbox_required,approved,created_by,created_at,updated_at FROM tunnels WHERE id=$1`, id))
	return t, e
}
func (d *DB) ListTunnels(status string, limit int) ([]*Tunnel, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id,task_id,finding_id,name,kind,status,target,listen_addr,remote_addr,credentials_ref,command,dry_run_script,evidence,sandbox_required,approved,created_by,created_at,updated_at FROM tunnels`
	var args []any
	if status != "" {
		args = []any{status}
		q += ` WHERE status=$1`
	}
	q += fmt.Sprintf(` ORDER BY updated_at DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)
	rows, e := d.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []*Tunnel
	for rows.Next() {
		t, e := scanTunnel(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (d *DB) UpdateTunnelStatus(id int64, status string, approved bool) error {
	_, e := d.Exec(`UPDATE tunnels SET status=$2,approved=$3,updated_at=now() WHERE id=$1`, id, status, approved)
	return e
}

// UpdateTunnelRuntime persists the authoritative process state. A tunnel is
// only shown as active after its listener has actually bound a port.
func (d *DB) UpdateTunnelRuntime(id int64, status, listenAddr, command string, evidence json.RawMessage) error {
	_, err := d.Exec(`UPDATE tunnels SET status=$2,listen_addr=$3,command=$4,evidence=$5,updated_at=now() WHERE id=$1`, id, status, listenAddr, command, evidence)
	return err
}
func (d *DB) DeleteTunnel(id int64) error {
	_, e := d.Exec(`DELETE FROM tunnels WHERE id=$1`, id)
	return e
}

var _ = sql.ErrNoRows
