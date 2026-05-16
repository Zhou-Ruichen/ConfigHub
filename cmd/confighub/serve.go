package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ruichen/config-hub/internal/server"
	"github.com/spf13/cobra"
)

const exitAuth = 21

func newServeCommand() *cobra.Command {
	var bindFlag, rootFlag string
	var allowNoToken bool
	cmd := &cobra.Command{
		Use:     "serve [--bind <addr>] [--root <dir>] [--allow-no-token]",
		Short:   "Run the ConfigHub web UI and HTTP API",
		Example: "confighub serve --bind 127.0.0.1:8787 --root examples",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("serve does not accept positional arguments")}
			}
			loopback := server.IsLoopbackBind(bindFlag)
			if allowNoToken && !loopback {
				return &exitError{code: exitUsage, err: fmt.Errorf("--allow-no-token is only allowed on loopback binds")}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			srv, err := server.NewWithConfig(server.Config{RootDir: root, LoopbackOnly: loopback, AllowNoToken: allowNoToken, Version: version})
			if err != nil {
				if errors.Is(err, server.ErrNoTokenConfigured) {
					return &exitError{code: exitAuth, err: err}
				}
				return err
			}
			if !loopback {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: non-loopback bind over plain HTTP; terminate TLS at a reverse proxy (see ops/caddy/Caddyfile.example) before serving real traffic")
			}
			httpSrv := &http.Server{Addr: bindFlag, Handler: srv.Handler()}
			errCh := make(chan error, 1)
			go func() {
				if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errCh <- err
					return
				}
				errCh <- nil
			}()
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
			select {
			case sig := <-stop:
				fmt.Fprintf(cmd.ErrOrStderr(), "shutting down after %s\n", sig)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := httpSrv.Shutdown(ctx); err != nil {
					return err
				}
				return <-errCh
			case err := <-errCh:
				return err
			}
		},
	}
	cmd.Flags().StringVar(&bindFlag, "bind", "127.0.0.1:8787", "address to bind")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	cmd.Flags().BoolVar(&allowNoToken, "allow-no-token", false, "allow startup without tokens on loopback binds")
	return cmd
}
