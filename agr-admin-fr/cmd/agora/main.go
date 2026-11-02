// agora es la CLI de administración de AGORA (agr-admin-fr).
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"agora-admin/internal/commands"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := commands.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
