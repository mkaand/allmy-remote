package main

import (
	"context"
	"errors"
	"log"
	"io"
	"strings"
	"os"
	"os/signal"
	"syscall"

	"github.com/mkaand/allmy-remote/internal/core"
)

func main() {
	cfg, err := core.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) == 2 && os.Args[1] == "init-admin" {
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 259))
		if err != nil || len(raw) > 258 {
			log.Fatal("invalid password input")
		}
		password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
		store, err := core.OpenStore(ctx, cfg.DataDir)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		if err := store.InitAdmin(ctx, password); err != nil {
			log.Fatal(err)
		}
		log.Print("administrator initialized")
		return
	}
	if len(os.Args) != 1 {
		log.Fatal("unknown command")
	}

	if err := core.Run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
