package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-port-forward/internal/config"
	"go-port-forward/internal/firewall"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/logger"
	"go-port-forward/internal/storage"
	"go-port-forward/internal/svc"
	"go-port-forward/internal/web"
	"go-port-forward/pkg/gc"
	pkglogger "go-port-forward/pkg/logger"
	"go-port-forward/pkg/pool"
)

const (
	serviceName    = "go-port-forward"
	serviceDisplay = "Go Port Forward"
	serviceDesc    = "Cross-platform TCP/UDP port forwarder with web UI"
)

func main() {
	var (
		configPath = flag.String("config", "", "path to config.yaml (default: next to executable)")
		serviceCmd = flag.String("service", "", "service command: install | uninstall | run")
	)
	flag.Parse()

	// Service install/uninstall don't need the full app to start.
	sc := svc.Config{Name: serviceName, DisplayName: serviceDisplay, Description: serviceDesc}
	switch *serviceCmd {
	case "install":
		if err := svc.Install(sc); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "install service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service installed.")
		return
	case "uninstall":
		if err := svc.Uninstall(sc); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "uninstall service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service uninstalled.")
		return
	}

	// --- Normal / "run" path ---
	app := &application{configPath: *configPath}

	if *serviceCmd == "run" {
		// Hand control to service manager (blocks until stopped by OS).
		if err := svc.Run(sc, app); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "service run: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Interactive foreground run.
	if err := app.Start(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := app.Stop(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "stop: %v\n", err)
	}
}

// application wires all subsystems together and implements svc.Runner.
type application struct {
	configPath string
	cfg        *config.AppConfig
	store      storage.Store
	mgr        *forward.Manager
	webSrv     *web.Server
	gcSvc      *gc.Service
}

func (a *application) Start() error {
	// Config
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	a.cfg = cfg

	// Logger
	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	logger.S.Infow("starting", "name", serviceDisplay)

	// Bridge internal logger to pkg/logger so pkg/gc etc. can log
	pkglogger.SetLogger(logger.L)

	// Goroutine pool (global, used by forward and gc)
	poolSize := cfg.Pool.Size
	if poolSize <= 0 {
		poolSize = 10000
	}
	if err := pool.InitGoroutinePool(poolSize, cfg.Pool.PreAlloc); err != nil {
		return fmt.Errorf("goroutine pool: %w", err)
	}
	logger.S.Infow("goroutine pool initialized", "size", poolSize, "preAlloc", cfg.Pool.PreAlloc)

	// Storage
	store, err := storage.Open(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	a.store = store

	// Forward manager
	mgr, err := forward.NewManager(store, cfg.Forward)
	if err != nil {
		return fmt.Errorf("forward manager: %w", err)
	}
	a.mgr = mgr

	// GC service
	gcCfg := &gc.Config{
		Enabled:          cfg.GC.Enabled,
		Interval:         time.Duration(cfg.GC.IntervalSeconds) * time.Second,
		Strategy:         gc.StrategyType(cfg.GC.Strategy),
		MemoryThreshold:  uint64(cfg.GC.MemoryThresholdMB) * 1024 * 1024,
		EnableStats:      true,
		EnableMonitoring: cfg.GC.EnableMonitoring,
		MaxRetries:       2,
		RetryInterval:    10 * time.Second,
		ExecutionTimeout: 60 * time.Second,
	}
	gcSvc, err := gc.NewService(gcCfg)
	if err != nil {
		logger.S.Warnw("GC service init failed, continuing without GC management", "err", err)
	} else {
		if err := gcSvc.Start(); err != nil {
			logger.S.Warnw("GC service start failed", "err", err)
		} else {
			a.gcSvc = gcSvc
			logger.S.Infow("GC service started",
				"strategy", cfg.GC.Strategy,
				"interval", gcCfg.Interval)
		}
	}

	// Web server
	fw := firewall.New()
	srv := web.New(cfg.Web, mgr, fw)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("web server: %w", err)
	}
	a.webSrv = srv

	return nil
}

func (a *application) Stop() error {
	logger.S.Info("shutting down …")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if a.webSrv != nil {
		_ = a.webSrv.Shutdown(ctx)
	}
	if a.mgr != nil {
		a.mgr.Shutdown()
	}
	if a.gcSvc != nil {
		_ = a.gcSvc.Stop()
	}
	if a.store != nil {
		_ = a.store.Close()
	}

	// Release global goroutine pool
	pool.Release()

	logger.Sync()
	pkglogger.Sync()
	return nil
}
