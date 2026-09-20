// Command localdemo runs YAK8SUI with controlled local OIDC and Kubernetes
// list/watch data. It is a developer demo and must not be used in production.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ron96g/yak8sui/backend/auth"
	"github.com/ron96g/yak8sui/backend/demo"
	"github.com/ron96g/yak8sui/backend/resources"
	appserver "github.com/ron96g/yak8sui/backend/server"
	"github.com/ron96g/yak8sui/backend/testidp"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func main() {
	if err := run(); err != nil {
		slog.Error("local demo stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	appAddress := flag.String("address", "127.0.0.1:19080", "loopback address for YAK8SUI")
	appURL := flag.String("public-url", "http://127.0.0.1:19080", "browser URL for YAK8SUI")
	idpAddress := flag.String("idp-address", "127.0.0.1:19081", "loopback address for the controlled OIDC provider")
	idpURL := flag.String("idp-url", "http://127.0.0.1:19081", "issuer URL for the controlled OIDC provider")
	databasePath := flag.String("database", ".local/demo/resources.db", "demo SQLite projection path")
	webDirectory := flag.String("web-directory", "frontend/dist", "built frontend directory")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	provider, err := testidp.New(*idpURL, demo.Users(), demo.UserName)
	if err != nil {
		return fmt.Errorf("create controlled identity provider: %w", err)
	}
	idpServer := &http.Server{Addr: *idpAddress, Handler: provider.Handler(), ReadHeaderTimeout: 5 * time.Second}
	if startErr := startServer(idpServer); startErr != nil {
		return fmt.Errorf("start controlled identity provider: %w", startErr)
	}
	defer shutdown(idpServer)

	cfg := demo.Config(*appAddress, *appURL, *idpURL, *databasePath, *webDirectory)
	if validateErr := cfg.Validate(); validateErr != nil {
		return fmt.Errorf("validate local demo: %w", validateErr)
	}
	oidc, err := auth.NewOIDC(ctx, &cfg.Identity)
	if err != nil {
		return fmt.Errorf("connect controlled identity provider: %w", err)
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(*databasePath), 0o750); mkdirErr != nil {
		return fmt.Errorf("create demo data directory: %w", mkdirErr)
	}
	projection, err := resources.Open(*databasePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := projection.Close(); closeErr != nil {
			slog.Error("close demo projection", "error", closeErr)
		}
	}()
	kube := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), demo.ListKinds(), demo.RuntimeObjects()...)
	observation, err := resources.NewRuntime(kube, demo.Discovery{}, &cfg, projection)
	if err != nil {
		return err
	}
	observation.SetRetryBounds(2*time.Second, 2*time.Second)
	go func() {
		if observeErr := observation.Run(ctx); observeErr != nil && ctx.Err() == nil {
			slog.Error("demo resource observation stopped", "error", observeErr)
		}
	}()

	appHandler := http.NewServeMux()
	appHandler.Handle("/", appserver.New(&cfg, oidc, auth.NewSessions(cfg.Server.SessionTTL), projection, observation.Catalog()).Handler())
	appHandler.HandleFunc("POST /demo/simulate/widget-update", demo.LiveSimulationHandler(kube))
	appHandler.HandleFunc("POST /demo/simulate/search-update", demo.SearchSimulationHandler(kube))
	appHandler.HandleFunc("POST /demo/simulate/observation-loss", demo.ObservationSimulationHandler(observation, kube))
	appHandler.HandleFunc("POST /demo/simulate/ownership-update", demo.OwnershipSimulationHandler(kube))
	appHandler.HandleFunc("POST /demo/simulate/label-join-update", demo.LabelJoinSimulationHandler(kube))
	appServer := &http.Server{Addr: *appAddress, Handler: appHandler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	listener, err := net.Listen("tcp", appServer.Addr)
	if err != nil {
		return fmt.Errorf("listen for YAK8SUI: %w", err)
	}
	slog.Info("local demo ready", "url", *appURL, "user", "Demo Operator", "database", *databasePath)
	go func() {
		<-ctx.Done()
		shutdown(appServer)
	}()
	if err := appServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func startServer(server *http.Server) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("controlled identity provider stopped", "error", serveErr)
		}
	}()
	return nil
}

func shutdown(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("local demo shutdown failed", "error", err)
	}
}
