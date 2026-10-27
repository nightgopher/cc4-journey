package repo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"
)

type MCPCredential struct {
	ID          int64      `json:"id"`
	UserID      int        `json:"userId"`
	IssuedBy    int        `json:"issuedBy"`
	Label       string     `json:"label"`
	TokenPrefix string     `json:"tokenPrefix"`
	AccessMode  string     `json:"accessMode"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	RevokedAt   *time.Time `json:"revokedAt"`
}

const credentialColumns = `id,user_id,issued_by,label,token_prefix,access_mode,created_at,last_used_at,revoked_at`

func credentialScan(row interface{ Scan(...any) error }) (*MCPCredential, error) {
	var c MCPCredential
	err := row.Scan(&c.ID, &c.UserID, &c.IssuedBy, &c.Label, &c.TokenPrefix, &c.AccessMode, &c.CreatedAt, &c.LastUsedAt, &c.RevokedAt)
	return &c, err
}

func MCPTokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func MintMCPCredential(ctx context.Context, userID, issuerID int, label, mode string) (*MCPCredential, string, error) {
	if (mode != "read_only" && mode != "read_write") || utf8.RuneCountInString(label) > 80 {
		return nil, "", fmt.Errorf("无效的接入模式或名称")
	}
	secret := make([]byte, 48)
	if _, err := rand.Read(secret); err != nil {
		return nil, "", err
	}
	raw := "cc4_mcp_" + base64.RawURLEncoding.EncodeToString(secret)
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	var locked int
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&locked); err != nil {
		return nil, "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE mcp_credentials SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return nil, "", err
	}
	c, err := credentialScan(tx.QueryRow(ctx, `INSERT INTO mcp_credentials(user_id,issued_by,label,token_hash,token_prefix,access_mode)
		VALUES($1,$2,$3,$4,$5,$6) RETURNING `+credentialColumns, userID, issuerID, label, MCPTokenHash(raw), raw[:16], mode))
	if err != nil {
		return nil, "", err
	}
	detail, _ := json.Marshal(map[string]any{"userId": userID, "credentialId": c.ID, "accessMode": mode})
	if _, err := tx.Exec(ctx, `INSERT INTO agent_tool_audit(actor,action,detail) VALUES($1,'mcp_credential_created',$2)`, fmt.Sprintf("user:%d", issuerID), string(detail)); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return c, raw, nil
}

func FindMCPCredentialByToken(ctx context.Context, raw string) (*MCPCredential, error) {
	return credentialScan(Pool.QueryRow(ctx, `SELECT `+credentialColumns+` FROM mcp_credentials WHERE token_hash=$1 AND revoked_at IS NULL`, MCPTokenHash(raw)))
}

func ActiveMCPCredential(ctx context.Context, id int64) (*MCPCredential, error) {
	return credentialScan(Pool.QueryRow(ctx, `SELECT `+credentialColumns+` FROM mcp_credentials WHERE id=$1 AND revoked_at IS NULL`, id))
}

func LatestMCPCredential(ctx context.Context, userID int) (*MCPCredential, error) {
	return credentialScan(Pool.QueryRow(ctx, `SELECT `+credentialColumns+` FROM mcp_credentials WHERE user_id=$1 ORDER BY id DESC LIMIT 1`, userID))
}

func RevokeMCPCredential(ctx context.Context, id int64, actorID int, admin bool) error {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ownerID int
	if err := tx.QueryRow(ctx, `SELECT user_id FROM mcp_credentials WHERE id=$1 FOR UPDATE`, id).Scan(&ownerID); err != nil {
		return err
	}
	if ownerID != actorID && !admin {
		return ErrForbidden
	}
	if _, err := tx.Exec(ctx, `UPDATE mcp_credentials SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1`, id); err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"userId": ownerID, "credentialId": id})
	if _, err := tx.Exec(ctx, `INSERT INTO agent_tool_audit(actor,action,detail) VALUES($1,'mcp_credential_revoked',$2)`, fmt.Sprintf("user:%d", actorID), string(detail)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
