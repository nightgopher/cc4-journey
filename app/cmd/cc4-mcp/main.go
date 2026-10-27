package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	ccmcp "github.com/nightgopher/cc4-journey/app/internal/mcp"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

// cc4-mcp：MCP server 独立进程。
//
// 用法：
//
//	cc4-mcp            # stdio 模式（配置到 EasyCode / Claude Code 的 .mcp.json）
//	cc4-mcp -http :3102  # Streamable HTTP 模式（远程 agent，可选 -token 校验）
func main() {
	httpAddr := os.Getenv("MCP_HTTP_ADDR")
	token := os.Getenv("MCP_TOKEN")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 数据库连接（查询类工具需要；失败则只读工具不可用但 dry-run 仍可工作——
	// 简化处理：直接失败退出，部署时保证库可用）。
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		u := &url.URL{Scheme: "postgres", User: url.UserPassword(getenv("DB_USER", "root"), getenv("DB_PASSWORD", "")), Host: net.JoinHostPort(getenv("DB_HOST", "localhost"), getenv("DB_PORT", "5433")), Path: getenv("DB_NAME", "callcenter_db")}
		dsn = u.String()
	}
	if err := repo.Connect(ctx, dsn); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer repo.Close()

	if httpAddr != "" {
		log.Printf("[cc4-mcp] HTTP listening on %s", httpAddr)
		log.Fatal(http.ListenAndServe(httpAddr, ccmcp.HTTPHandler(token)))
		return
	}
	// stdio 模式：日志必须走 stderr，stdout 是协议通道。
	if err := ccmcp.ServeStdio(); err != nil {
		log.Fatalf("stdio server failed: %v", err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
