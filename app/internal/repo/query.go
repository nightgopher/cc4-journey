package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Object is used for read models whose fields are defined by a fixed SQL projection.
type Object map[string]any

func decodeObject(raw []byte) (Object, error) {
	var row Object
	err := json.Unmarshal(raw, &row)
	return row, err
}

type DBTX interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func Objects(ctx context.Context, db DBTX, sql string, args ...any) ([]Object, error) {
	rows, err := db.Query(ctx, `SELECT to_jsonb(q) FROM (`+sql+`) q`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Object{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var row Object
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func One(ctx context.Context, db DBTX, sql string, args ...any) (Object, error) {
	rows, err := Objects(ctx, db, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, pgx.ErrNoRows
	}
	return rows[0], nil
}

func affected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateObject only accepts identifiers from a caller-owned allowlist. Values remain parameters.
func UpdateObject(ctx context.Context, db DBTX, table string, id int, fields Object, allowed map[string]string) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if _, ok := allowed[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return errors.New("没有可更新的字段")
	}
	args := []any{id}
	sets := []string{}
	for _, key := range keys {
		args = append(args, fields[key])
		expr := fmt.Sprintf("$%d", len(args))
		if allowed[key] == "date" {
			expr = "NULLIF(" + expr + "::text,'')::date"
		}
		if allowed[key] == "jsonb" {
			b, err := json.Marshal(fields[key])
			if err != nil {
				return err
			}
			args[len(args)-1] = string(b)
			expr += "::jsonb"
		}
		sets = append(sets, key+"="+expr)
	}
	tag, err := db.Exec(ctx, `UPDATE `+table+` SET `+strings.Join(sets, ",")+`,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, args...)
	return affected(tag, err)
}
