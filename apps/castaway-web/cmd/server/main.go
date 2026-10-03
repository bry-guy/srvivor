package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("castaway-web: %v", err)
	}
}

func run() error {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if cfg.AutoMigrate {
		if err := app.RunMigrations(ctx, pool, cfg.MigrationsDir); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
	}

	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{
		Enabled:      cfg.ServiceAuthEnabled,
		BearerTokens: cfg.ServiceAuthBearerTokens,
		Principal:    cfg.ServiceAuthPrincipal,
	}), httpapi.WithBootstrapAdminDiscordUserID(cfg.BootstrapAdminDiscordUserID), httpapi.WithPublic(httpapi.PublicConfig{
		BaseURL:             cfg.PublicBaseURL,
		DiscordClientID:     cfg.DiscordClientID,
		DiscordClientSecret: cfg.DiscordClientSecret,
		InstanceID:          cfg.PublicInstanceID,
		LeagueName:          cfg.PublicLeagueName,
		LeagueInstanceIDs:   cfg.PublicLeagueIDs,
	}))
	router := server.Router()
	go server.RunButtonResolver(ctx)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// The public listener is what the Cloudflare tunnel reaches; the internal one (service token) stays in-cluster.
	var publicServer *http.Server
	if cfg.PublicPort != "" {
		publicServer = &http.Server{Addr: ":" + cfg.PublicPort, Handler: server.PublicRouter(), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			log.Printf("castaway-web public listener on :%s", cfg.PublicPort)
			if err := publicServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("public http server error: %v", err)
			}
		}()
	}

	shutdownDone := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if publicServer != nil {
			if err := publicServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("public server shutdown error: %v", err)
			}
		}
		if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("server shutdown error: %v", shutdownErr)
		}
		close(shutdownDone)
	}()

	log.Printf("castaway-web starting %s", buildinfo.String())
	log.Printf("castaway-web listening on :%s", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}
	<-shutdownDone
	return nil
}
