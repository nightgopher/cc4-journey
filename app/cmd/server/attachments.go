package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nightgopher/cc4-journey/app/internal/storage"
)

// migrateAttachments copies UPLOAD_DIR attachments into TOS and verifies every
// object byte for byte. Without --apply it only counts what would be copied.
// Local files are never deleted; remove them manually after verifying.
func migrateAttachments(store *storage.Store, args []string) {
	flags := flag.NewFlagSet("migrate-attachments", flag.ExitOnError)
	apply := flags.Bool("apply", false, "实际上传并逐字节校验；默认只统计")
	_ = flags.Parse(args)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	counts, err := store.Migrate(ctx, *apply)
	if counts != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"apply": *apply, "counts": counts})
	}
	if err != nil {
		log.Fatal(err)
	}
}
