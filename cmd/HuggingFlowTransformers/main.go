package main

import (
	"context"
	"embed"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"huggingflowtransformers/internal/brandlog"
	"huggingflowtransformers/internal/config"
	"huggingflowtransformers/internal/device"
	"huggingflowtransformers/internal/engine"
	"huggingflowtransformers/internal/localproxy"
	"huggingflowtransformers/internal/report"
	"huggingflowtransformers/internal/runtime"
	"huggingflowtransformers/internal/supervisor"
	"huggingflowtransformers/internal/transport"
)

var (
	hftVersion  = "dev"
	buildCommit = "unknown"
	buildTime   = "unknown"
)

//go:embed embedded/engine.bin embedded/argv-scrubber.so
var embeddedFS embed.FS

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "HuggingFlowTransformers failed: %v\n", brandlog.Redact(err.Error()))
		os.Exit(1)
	}
}

func run(args []string) error {
	if isVersionCommand(args) {
		_, err := fmt.Fprintf(os.Stdout, "HuggingFlowTransformers v%s\n", hftVersion)
		return err
	}

	cfg, err := config.Load(args)
	if err != nil {
		return err
	}

	if cfg.Report {
		runtimePath, guardPath, err := releaseRuntimeAssets()
		if err != nil {
			return err
		}
		devices, err := device.Detect(cfg.Devices)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(os.Stdout, report.Render(report.Input{
			Config:             cfg,
			Devices:            devices,
			Version:            hftVersion,
			BuildCommit:        buildCommit,
			BuildTime:          buildTime,
			RuntimeReleasePath: runtimePath,
			ProcessGuardPath:   guardPath,
		}))
		return err
	}

	enginePath, scrubberPath, err := releaseRuntimeAssets()
	if err != nil {
		return err
	}

	logger := brandlog.New(brandlog.Options{
		Mode:    brandlog.Mode(cfg.LogMode),
		Version: hftVersion,
	})
	devices, err := device.Detect(cfg.Devices)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runtimeBaseURL := cfg.BaseURL
	if cfg.GatewayMode {
		proxy, err := localproxy.Start(ctx, localproxy.Options{
			Dial: func(ctx context.Context) (net.Conn, error) {
				conn, err := transport.Dialer{Config: cfg}.Dial(ctx)
				if err == nil || !cfg.UpstreamDirect {
					return conn, err
				}
				return transport.DialDirect(ctx, cfg.BaseURL, cfg.GatewayConnectTimeout)
			},
			IdleTimeout: cfg.GatewayIdleTimeout,
		})
		if err != nil {
			return fmt.Errorf("start HFT Secure Gateway mode: %w", err)
		}
		defer proxy.Close()
		runtimeBaseURL = proxy.RuntimeBaseURL()
	}

	runner := engine.NewRunner(engine.RunnerOptions{
		Path:        enginePath,
		BaseURL:     runtimeBaseURL,
		InternalKey: cfg.InternalKey,
		NodeName:    cfg.NodeName,
		CompatMode:  cfg.CompatMode,
		ArgvWrapper: scrubberPath,
		RawLogDir:   rawLogDir(cfg),
		RawLogHours: cfg.RawLogRetentionHours,
	})

	s := supervisor.New(supervisor.Config{
		Devices:            devices,
		NoModelDataTimeout: cfg.ModelDataTimeout,
		RestartOnExit:      cfg.RestartOnExit,
		Runner:             runner,
		EventSink:          eventAdapter{logger: logger, cfg: cfg},
	})
	if err := s.Start(ctx); err != nil {
		return err
	}
	s.Wait()
	_ = buildCommit
	_ = buildTime
	return nil
}

func isVersionCommand(args []string) bool {
	if len(args) != 2 {
		return false
	}
	return args[1] == "--version" || args[1] == "version"
}

func releaseRuntimeAssets() (string, string, error) {
	engineBytes, err := embeddedFS.ReadFile("embedded/engine.bin")
	if err != nil {
		return "", "", fmt.Errorf("embedded runtime missing")
	}
	enginePath, err := runtime.Release(runtime.Options{
		Version: hftVersion,
		Name:    "HuggingFlowTransformers-runtime",
		Bytes:   engineBytes,
	})
	if err != nil {
		return "", "", err
	}
	scrubberBytes, err := embeddedFS.ReadFile("embedded/argv-scrubber.so")
	if err != nil {
		return "", "", fmt.Errorf("embedded process wrapper missing")
	}
	scrubberPath, err := runtime.Release(runtime.Options{
		Version: hftVersion,
		Name:    "HuggingFlowTransformers-process-wrapper.so",
		Mode:    0o600,
		Bytes:   scrubberBytes,
	})
	if err != nil {
		return "", "", err
	}
	return enginePath, scrubberPath, nil
}

func rawLogDir(cfg config.Config) string {
	if cfg.LogMode != config.LogDebug {
		return ""
	}
	return cfg.DebugDir
}

type eventAdapter struct {
	logger *brandlog.Logger
	cfg    config.Config
}

func (a eventAdapter) Event(event supervisor.Event) {
	a.logger.Event(brandlog.Event{
		Time:   time.Now().UTC(),
		Level:  event.Level,
		Device: event.Device,
		Node:   a.cfg.NodeName(event.Device),
		Name:   event.Name,
	})
}
