// Command gameexplorer runs the GameExplorer HTTP API (and serves the SPA).
//
// This is the composition root: configuration, adapters and bounded contexts
// are wired here and nowhere else.
//
// Usage:
//
//	gameexplorer                 run the server (configured by environment)
//	gameexplorer hash-password   read a password from stdin, print its argon2id hash
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	identityinfra "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/infrastructure"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpserver"
)

const shutdownTimeout = 30 * time.Second

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		if err := hashPassword(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	bootLog := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(nil)
	if err != nil {
		bootLog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(cfg, log); err != nil {
		log.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := newApp(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	srv := httpserver.New(net.JoinHostPort("", strconv.Itoa(cfg.Port)), a.handler)

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "library", cfg.LibraryPath, "data", cfg.DataPath,
			"uid", os.Getuid(), "gid", os.Getgid())
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// hashPassword reads one line from in and writes its argon2id PHC hash to out.
func hashPassword(in io.Reader, out io.Writer) error {
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		return errors.New("empty password: pipe it on stdin, e.g. echo 'my password' | gameexplorer hash-password")
	}
	phc, err := identityinfra.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, phc)
	return err
}
