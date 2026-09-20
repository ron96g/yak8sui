// Command testfixture runs a controlled local OIDC provider and YAK8SUI for browser tests.
// It is not intended for production deployment.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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

const (
	idpIssuerURL  = "http://127.0.0.1:18081"
	readinessPath = idpIssuerURL + "/.well-known/openid-configuration"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	provider, err := testidp.New(idpIssuerURL, demo.Users(), demo.UserName)
	if err != nil {
		return err
	}
	idpServer := &http.Server{Addr: ":18081", Handler: provider.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go serve(idpServer)
	if readinessErr := waitForIDP(); readinessErr != nil {
		return readinessErr
	}
	databasePath := filepath.Join(os.TempDir(), fmt.Sprintf("yak8sui-browser-%d.db", os.Getpid()))
	defer func() {
		if removeErr := os.Remove(databasePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Printf("remove browser projection: %v", removeErr)
		}
	}()
	cfg := demo.Config(":18080", "http://127.0.0.1:18080", idpIssuerURL, databasePath, "dist")
	cfg.Server.SessionTTLRaw = "5m"
	if validateErr := cfg.Validate(); validateErr != nil {
		return validateErr
	}
	oidc, err := auth.NewOIDC(ctx, &cfg.Identity)
	if err != nil {
		return err
	}
	projection, err := resources.Open(databasePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := projection.Close(); closeErr != nil {
			log.Printf("close browser projection: %v", closeErr)
		}
	}()
	kube := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), demo.ListKinds(), demo.RuntimeObjects()...)
	observation, err := resources.NewRuntime(kube, demo.Discovery{}, &cfg, projection)
	if err != nil {
		return err
	}
	observation.SetRetryBounds(500*time.Millisecond, 2*time.Second)
	go func() {
		if observeErr := observation.Run(ctx); observeErr != nil && ctx.Err() == nil {
			log.Printf("resource observer stopped: %v", observeErr)
		}
	}()
	appHandler := http.NewServeMux()
	appHandler.Handle("/", appserver.New(&cfg, oidc, auth.NewSessions(cfg.Server.SessionTTL), projection, observation.Catalog()).Handler())
	appHandler.HandleFunc("POST /demo/simulate/widget-update", demo.LiveSimulationHandler(kube))
	appHandler.HandleFunc("POST /demo/simulate/search-update", demo.SearchSimulationHandler(kube))
	appHandler.HandleFunc("POST /demo/simulate/observation-loss", demo.ObservationSimulationHandler(observation, kube))
	appHandler.HandleFunc("POST /demo/simulate/ownership-update", demo.OwnershipSimulationHandler(kube))
	appHandler.HandleFunc("POST /demo/simulate/label-join-update", demo.LabelJoinSimulationHandler(kube))
	appServer := &http.Server{Addr: cfg.Server.Address, Handler: appHandler, ReadHeaderTimeout: 5 * time.Second}
	go serve(appServer)
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(appServer.Shutdown(shutdown), idpServer.Shutdown(shutdown))
}

func serve(server *http.Server) {
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func waitForIDP() error {
	for range 50 {
		response, err := http.Get(readinessPath)
		if err == nil {
			closeErr := response.Body.Close()
			if response.StatusCode == 200 {
				if closeErr != nil {
					return fmt.Errorf("close identity provider readiness response: %w", closeErr)
				}
				return nil
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.New("controlled identity provider did not start")
}
