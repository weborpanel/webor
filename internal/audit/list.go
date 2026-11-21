package audit

import (
	"database/sql"
	"encoding/json"
)

// Event is one stored audit entry.
type Event struct {
	ID        int64           `json:"id"`
	TS        string          `json:"ts"`
	Actor     string          `json:"actor"`
	Operation string          `json:"operation"`
	Args      json.RawMessage `json:"args"`
	Result    string          `json:"result"`
}

// List returns events newest first. before > 0 pages to ids below it.
func List(db *sql.DB, before int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, ts, actor, operation, args_json, result FROM audit_events`
	args := []any{}
	if before > 0 {
		q += ` WHERE id < ?`
		args = append(args, before)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var argsJSON string
		if err := rows.Scan(&e.ID, &e.TS, &e.Actor, &e.Operation, &argsJSON, &e.Result); err != nil {
			return nil, err
		}
		if json.Valid([]byte(argsJSON)) {
			e.Args = json.RawMessage(argsJSON)
		} else {
			e.Args = json.RawMessage(`{}`)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
