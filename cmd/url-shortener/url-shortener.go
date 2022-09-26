// Grüezi!
// This service reduces long links from sites on the Internet.
// If you have any questions, email the author Ilya Scheblanov <ilya.scheblanov@gmail.com>.
// If you like the project or use it for any purpose, don't hesitate to give it a star on GitHub! ;)
package main

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"go.uber.org/zap"

	"github.com/illyasch/url-shortener/cmd/url-shortener/handlers"
	"github.com/illyasch/url-shortener/pkg/data/database"
	"github.com/illyasch/url-shortener/pkg/sys/logger"
)

const configPrefix = "SHORTENER"

type config struct {
	conf.Version
	DB struct {
		User         string `conf:"default:postgres"`
		Password     string `conf:"default:postgres,mask"`
		Host         string `conf:"default:localhost"`
		Name         string `conf:"default:postgres"`
		MaxIdleConns int    `conf:"default:0"`
		MaxOpenConns int    `conf:"default:0"`
		DisableTLS   bool   `conf:"default:true"`
	}
	Web struct {
		ReadTimeout     time.Duration `conf:"default:5s"`
		WriteTimeout    time.Duration `conf:"default:10s"`
		IdleTimeout     time.Duration `conf:"default:120s"`
		ShutdownTimeout time.Duration `conf:"default:20s"`
		APIHost         string        `conf:"default:0.0.0.0:3000"`
	}
}

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

func main() {
	log, err := logger.New("shortener")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	if err := run(log); err != nil {
		log.Errorw("startup", "ERROR", err)
		_ = log.Sync()
		os.Exit(1)
	}
}

func run(logger *zap.SugaredLogger) error {
	cfg, err := parseConfig(configPrefix, logger)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	// =========================================================================
	// App Starting

	logger.Infow("starting service", "version", build)
	defer logger.Infow("shutdown complete")
	expvar.NewString("build").Set(build)

	// =========================================================================
	// Database Support

	// Create connectivity to the database.
	logger.Infow("startup", "status", "initializing database support", "host", cfg.DB.Host)

	db, err := database.Open(database.Config{
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		Host:         cfg.DB.Host,
		Name:         cfg.DB.Name,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		DisableTLS:   cfg.DB.DisableTLS,
	})
	if err != nil {
		return fmt.Errorf("connecting to db: %w", err)
	}
	defer func() {
		logger.Infow("shutdown", "status", "stopping database support", "host", cfg.DB.Host)
		if err := db.Close(); err != nil {
			logger.Errorw("shutdown", "ERROR", fmt.Errorf("db close: %w", err))
		}
	}()

	// =========================================================================
	// Start API Service

	logger.Infow("startup", "status", "initializing V1 API support")

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Construct the mux for the API calls.
	apiMux := handlers.APIConfig{
		DB:  db,
		Log: logger,
	}.Router()

	// Construct a server to service the requests against the mux.
	srv := http.Server{
		Addr:         cfg.Web.APIHost,
		Handler:      apiMux,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     zap.NewStdLog(logger.Desugar()),
	}

	// Make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// Start the service listening for srv requests.
	go func() {
		logger.Infow("startup", "status", "srv router started", "host", srv.Addr)
		serverErrors <- srv.ListenAndServe()
	}()

	// =========================================================================
	// Shutdown

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		logger.Infow("shutdown", "status", "shutdown started", "signal", sig)
		defer logger.Infow("shutdown", "status", "shutdown complete", "signal", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		// Asking listener to shut down and shed load.
		if err := srv.Shutdown(ctx); err != nil {
			if cErr := srv.Close(); cErr != nil {
				logger.Errorw("shutdown", "ERROR", fmt.Errorf("server close: %w", cErr))
			}
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}

func parseConfig(prefix string, logger *zap.SugaredLogger) (config, error) {
	cfg := config{
		Version: conf.Version{
			Build: build,
			Desc:  "Copyright Ilya Scheblanov",
		},
	}

	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		fmt.Println(help)
		return cfg, err
	}

	out, err := conf.String(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("generating config for output: %w", err)
	}
	logger.Infow("startup", "config", out)

	return cfg, nil
}
