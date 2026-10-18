package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/zelport/zelport/internal/endpoint"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/web"
)

// Set at release build time with -ldflags "-X main.version=...".
var version = "dev"
var builtAt = "unknown"
var factories []func() feature.Module

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	return runWithRuntime(func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	}, listen)
}
func runWithRuntime(newContext func() (context.Context, context.CancelFunc), openListeners func(int) ([]net.Listener, error)) error {
	started := time.Now()
	port := flag.Int("port", 8080, "HTTP reverse proxy and administration port")
	config := flag.String("config", "", "configuration file (default: XDG config on macOS/Linux, native user config directory elsewhere)")
	allowRoot := flag.Bool("allow-root", os.Getenv(allowRootEnv) == "1", "allow running as root (env: ZELPORT_ALLOW_ROOT=1; warns on startup)")
	showVersion := flag.Bool("version", false, "print version and exit")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flag.NArg() != 0 {
		return errors.New("unexpected arguments; use zelport --help")
	}
	if *showVersion {
		fmt.Printf("zelport %s (built %s)\n", version, builtAt)
		return nil
	}
	if err := checkRoot(effectiveUID(), *allowRoot, os.Stderr); err != nil {
		return err
	}
	if *port < 1 || *port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	path, err := resolveConfigPath(*config)
	if err != nil {
		return fmt.Errorf("configuration path: %w", err)
	}
	*config = path
	var modules []feature.Module
	for _, factory := range factories {
		modules = append(modules, factory())
	}
	m, err := app.New(*config, *port, modules...)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	listeners, err := openListeners(*port)
	if err != nil {
		return err
	}
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()
	configPath, err := filepath.Abs(*config)
	if err != nil {
		return err
	}
	addresses := make([]string, 0, len(listeners))
	for _, l := range listeners {
		addresses = append(addresses, l.Addr().String())
	}
	handler, err := web.New(m, *port, web.RuntimeInfo{Version: version, BuiltAt: builtAt, StartedAt: started, ConfigPath: configPath, Listeners: addresses})
	if err != nil {
		return err
	}
	m.SetAdministrationHandler(handler.ServeRemoteAdministration)
	defer handler.Close()
	ctx, cancel := newContext()
	defer cancel()
	if err := m.Start(ctx); err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20, BaseContext: func(net.Listener) context.Context { return ctx }}
	failures := make(chan error, len(listeners))
	for _, listener := range listeners {
		go func(l net.Listener) { failures <- server.Serve(l) }(listener)
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		path, err := endpoint.Path()
		if err == nil {
			err = endpoint.Publish(path, *port)
		}
		if err != nil {
			log.Printf("warning: cannot record local endpoint: %v", err)
		}
	}
	log.Printf("Zelport App %s", version)
	log.Printf("Build time: %s", builtAt)
	log.Printf("Zelport App is listening at http://localhost:%d (loopback only)", *port)
	log.Printf("Configuration: %s", *config)
	select {
	case <-ctx.Done():
	case err = <-failures:
		cancel()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	handler.Close() // Terminates all module-owned and upgraded connections.
	_ = server.Shutdown(shutdown)
	return err
}
func listen(port int) ([]net.Listener, error) {
	v4, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on localhost:%d: %w", port, err)
	}
	v6, err := net.Listen("tcp6", net.JoinHostPort("::1", strconv.Itoa(port)))
	if err != nil {
		if errors.Is(err, syscall.EAFNOSUPPORT) || errors.Is(err, syscall.EADDRNOTAVAIL) {
			return []net.Listener{v4}, nil
		}
		v4.Close()
		return nil, fmt.Errorf("cannot listen on [::1]:%d: %w", port, err)
	}
	return []net.Listener{v4, v6}, nil
}
