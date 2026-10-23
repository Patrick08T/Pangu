package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/identity"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/audit"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/web"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	mode := flag.String("mode", "all", "运行模式：all、api、worker 或 migrate")
	port := flag.Int("port", 8080, "API 回环监听端口")
	flag.Parse()
	if err := run(*mode, *port); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(mode string, port int) error {
	if mode != "all" && mode != "api" && mode != "worker" && mode != "migrate" {
		return fmt.Errorf("无效运行模式 %q", mode)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if mode == "migrate" {
		return database.Migrate(ctx)
	}
	client, err := hatchet.NewClient()
	if err != nil {
		return fmt.Errorf("连接 Hatchet: %w", err)
	}
	defer func() { _ = client.Close(context.Background()) }()
	if err := database.Check(ctx); err != nil {
		return err
	}
	backend := scheduler.NewBackend(client)
	go audit.Maintain(ctx, backend.Namespace())
	go backend.Maintain(ctx)
	go backend.MaintainHistory(ctx)
	if mode != "api" {
		go backend.MaintainRetention(ctx)
		worker, err := scheduler.NewWorker(client, agent.Execute, agent.Preflight)
		if err != nil {
			return err
		}
		stop, err := worker.Start()
		if err != nil {
			return err
		}
		defer func() { _ = stop() }()
		log.Printf("Pangu Worker 已就绪，工作流=%s", client.NewWorkflow(scheduler.WorkflowName).GetName())
	}
	if mode == "worker" {
		<-ctx.Done()
		return nil
	}
	return serve(ctx, web.Handler(api.New(backend, identity.Service{Namespace: backend.Namespace()})), port)
}

func serve(ctx context.Context, handler http.Handler, port int) error {
	server := &http.Server{
		Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: time.Minute,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	log.Printf("Pangu API: http://%s", server.Addr)
	select {
	case err := <-result:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(stopCtx); err != nil {
			return err
		}
	}
	return nil
}
