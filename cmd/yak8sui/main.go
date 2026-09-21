package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ron96g/yak8sui/backend/auth"
	"github.com/ron96g/yak8sui/backend/config"
	"github.com/ron96g/yak8sui/backend/resources"
	appserver "github.com/ron96g/yak8sui/backend/server"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/example.yaml", "path to deployment YAML")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var oidc *auth.OIDC
	if cfg.AllowAll() {
		slog.Warn("authentication is disabled; every visitor has access to every configured tenant and action")
	} else {
		oidc, err = auth.NewOIDC(ctx, &cfg.Identity)
		if err != nil {
			return fmt.Errorf("startup failed: %w", err)
		}
	}
	projection, err := resources.Open(cfg.Observation.DatabasePath)
	if err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}
	defer func() {
		if closeErr := projection.Close(); closeErr != nil {
			slog.Error("close SQLite projection", "error", closeErr)
		}
	}()
	kubeConfig, err := kubernetesConfig(cfg.Observation.Kubeconfig)
	if err != nil {
		return fmt.Errorf("startup failed: Kubernetes configuration: %w", err)
	}
	client, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("startup failed: Kubernetes client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("startup failed: Kubernetes discovery client: %w", err)
	}
	runtime, err := resources.NewRuntime(client, discoveryClient, &cfg, projection)
	if err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}
	go func() {
		if observeErr := runtime.Run(ctx); observeErr != nil && ctx.Err() == nil {
			slog.Error("resource observation stopped", "error", observeErr)
		}
	}()
	handler := appserver.New(&cfg, oidc, auth.NewSessions(cfg.Server.SessionTTL), projection, runtime.Catalog(), runtime.Mutator()).Handler()
	httpServer := &http.Server{Addr: cfg.Server.Address, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdown); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()
	slog.Info("YAK8SUI listening", "address", cfg.Server.Address)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func kubernetesConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
