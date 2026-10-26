package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/api"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

// 与存量 docker-compose 默认值一致的环境变量约定：
// PORT / DB_HOST / DB_PORT / DB_NAME / DB_USER / DB_PASSWORD / JWT_SECRET
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	dbHost := getenv("DB_HOST", "localhost")
	dbPort := getenv("DB_PORT", "5433")
	dbName := getenv("DB_NAME", "callcenter_db")
	dbUser := getenv("DB_USER", "root")
	dbPass := getenv("DB_PASSWORD", "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dsn := "postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName
	if err := repo.Connect(ctx, dsn); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer repo.Close()

	handler := api.Router(jwtSecret())
	addr := host + ":" + port
	log.Printf("[cc4-go] listening on %s (db=%s:%s/%s)", addr, dbHost, dbPort, dbName)
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "your_super_secret_jwt_key_here_change_in_production"
}
