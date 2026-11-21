package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-durable-execution-sdk-go/durable"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/config"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/store"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/workflow"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.FromEnv()
	ctx := context.Background()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("aws config", "err", err)
		os.Exit(1)
	}
	st := store.NewDynamo(dynamodb.NewFromConfig(awsCfg), cfg.TableName)
	durable.Start(workflow.New(st, cfg))
}
