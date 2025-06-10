package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notcoin_contest/internal/config"
	"notcoin_contest/internal/handler"
	"notcoin_contest/internal/service"
	"notcoin_contest/internal/store"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type application struct {
	config        *config.Config
	logger        *log.Logger
	db            *sql.DB
	redisClient   *redis.Client
	saleService   *service.SaleService
	server        *http.Server
	shutdownChan  chan struct{}
	schedulerDone chan struct{}
}

func main() {
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	if cfg.SaleCycleInterval <= 0 {
		logger.Fatalf("SaleCycleInterval must be a positive duration. Check configuration.")
	}

	db, err := store.ConnectDB(cfg.DBDriver, cfg.DBDataSourceName)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Printf("Error closing database: %v", err)
		}
	}()

	if err := store.RunMigrations(db, cfg.MigrationsDir); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}

	redisClient, err := store.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Printf("Error closing Redis client: %v", err)
		}
	}()

	dbStore := store.NewDBStore(db)
	redisStore := store.NewRedisStore(redisClient)
	saleService := service.NewSaleService(logger, dbStore, redisStore, cfg)

	app := &application{
		config:        cfg,
		logger:        logger,
		db:            db,
		redisClient:   redisClient,
		saleService:   saleService,
		shutdownChan:  make(chan struct{}),
		schedulerDone: make(chan struct{}),
	}

	go app.runSaleScheduler()

	mux := http.NewServeMux()
	checkoutHandler := handler.NewCheckoutHandler(logger, saleService)
	purchaseHandler := handler.NewPurchaseHandler(logger, saleService)

	mux.Handle("/checkout", checkoutHandler)
	mux.Handle("/purchase", purchaseHandler)

	app.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     logger,
	}

	app.serve()
}

func (app *application) serve() {
	app.logger.Printf("Starting server on %s", app.server.Addr)

	errChan := make(chan error)
	go func() {
		if err := app.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		app.logger.Fatalf("Server error: %v", err)
	case sig := <-quit:
		app.logger.Printf("Received signal %s. Shutting down server...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	app.logger.Println("Signaling sale scheduler to stop...")
	close(app.shutdownChan)
	select {
	case <-app.schedulerDone:
		app.logger.Println("Sale scheduler stopped.")
	case <-time.After(10 * time.Second):
		app.logger.Println("Sale scheduler did not stop in time.")
	}

	if err := app.server.Shutdown(ctx); err != nil {
		app.logger.Printf("Graceful server shutdown failed: %v", err)
	} else {
		app.logger.Println("Server gracefully stopped.")
	}

	app.logger.Println("Application shut down complete.")
}

func (app *application) runSaleScheduler() {
	defer close(app.schedulerDone)

	app.logger.Println("Scheduler: Running initial sale cycle management.")
	if err := app.saleService.ManageHourlySaleCycle(context.Background()); err != nil {
		app.logger.Printf("Scheduler: Error during initial sale cycle management: %v", err)
	}

	ticker := time.NewTicker(app.config.SaleCycleInterval)
	defer ticker.Stop()

	app.logger.Printf("Sale scheduler started. Will run every %s.", app.config.SaleCycleInterval.String())

	for {
		select {
		case <-ticker.C:
			app.logger.Println("Scheduler: Triggered by ticker. Running hourly sale cycle management.")
			if err := app.saleService.ManageHourlySaleCycle(context.Background()); err != nil {
				app.logger.Printf("Scheduler: Error during hourly sale cycle management: %v", err)
			}
		case <-app.shutdownChan:
			app.logger.Println("Scheduler: Received shutdown signal. Stopping...")
			return
		}
	}
}
