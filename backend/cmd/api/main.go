package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	lambdaapi "github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/catalog"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/config"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/httpapi"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/orchestrator"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/store"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/workflow"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.FromEnv()
	ctx := context.Background()

	st, orch, err := wire(ctx, cfg)
	if err != nil {
		slog.Error("startup", "err", err)
		os.Exit(1)
	}
	if err := catalog.Seed(ctx, st); err != nil {
		slog.Error("seed catalog", "err", err)
		os.Exit(1)
	}

	handler := httpapi.New(st, orch, cfg)
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(httpapi.LambdaAdapter(handler))
		return
	}

	server := &http.Server{Addr: cfg.ListenAddr, Handler: handler}
	go func() {
		slog.Info("api listening", "addr", cfg.ListenAddr, "mode", mode(cfg))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func wire(ctx context.Context, cfg config.Config) (store.Store, orchestrator.Orchestrator, error) {
	if cfg.TableName == "" {
		st := store.NewMemory()
		handle := workflow.New(st, cfg)
		return st, orchestrator.NewLocal(st, cfg, handle), nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, nil, err
	}
	st := store.NewDynamo(dynamodb.NewFromConfig(awsCfg), cfg.TableName)
	if cfg.DurableFunctionName == "" {
		return st, orchestrator.NewLocal(st, cfg, workflow.New(st, cfg)), nil
	}
	return st, orchestrator.NewAWS(lambdaapi.NewFromConfig(awsCfg), cfg.DurableFunctionName), nil
}

func mode(cfg config.Config) string {
	if cfg.DurableFunctionName == "" {
		return "local-durable"
	}
	return "aws-durable"
}
