package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"huggingflowtransformers/internal/brandlog"
	"huggingflowtransformers/internal/config"
	"huggingflowtransformers/internal/gateway"
)

var gatewayVersion = "dev"

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "HuggingFlowTransformers Gateway failed: %v\n", brandlog.Redact(err.Error()))
		os.Exit(1)
	}
}

func run(args []string) error {
	if isVersionCommand(args) {
		_, err := fmt.Fprintf(os.Stdout, "HuggingFlowTransformers Gateway v%s\n", gatewayVersion)
		return err
	}

	cfg, err := config.LoadGateway(args)
	if err != nil {
		return err
	}
	logger := brandlog.New(brandlog.Options{Mode: brandlog.Mode(cfg.LogMode), Version: gatewayVersion})
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return gateway.New(cfg, logger).Serve(ctx)
}

func isVersionCommand(args []string) bool {
	if len(args) != 2 {
		return false
	}
	return args[1] == "--version" || args[1] == "version"
}
