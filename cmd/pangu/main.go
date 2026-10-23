package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func main() {
	mode := flag.String("mode", "all", "运行模式：all、api 或 worker")
	port := flag.Int("port", 8080, "API 回环监听端口")
	flag.Parse()
	if err := run(*mode, *port); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(mode string, port int) error {
	if mode != "all" && mode != "api" && mode != "worker" {
		return fmt.Errorf("无效运行模式 %q", mode)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client, err := hatchet.NewClient()
	if err != nil {
		return fmt.Errorf("连接 Hatchet: %w", err)
	}
	defer func() { _ = client.Close(context.Background()) }()
	if mode != "api" {
		worker, err := scheduler.NewWorker(client, agent.Execute)
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
	return serve(ctx, api.New(scheduler.NewBackend(client)), port)
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
