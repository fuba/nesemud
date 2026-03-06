package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"nes_recorder/internal/config"
	"nes_recorder/internal/daemon"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nesd <serve|daemon> [--config path]")
		os.Exit(2)
	}
	sub := os.Args[1]
	fs := flag.NewFlagSet(sub, flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to config json")
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse flags failed: %v\n", err)
		os.Exit(2)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	logger, closer, err := buildLogger(cfg.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log init failed: %v\n", err)
		os.Exit(1)
	}
	defer closer()

	switch sub {
	case "daemon":
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve executable failed: %v\n", err)
			os.Exit(1)
		}
		args := []string{"serve"}
		if *cfgPath != "" {
			args = append(args, "--config", *cfgPath)
		}
		if err := daemon.StartDetached(exe, args, cfg.LogFile); err != nil {
			fmt.Fprintf(os.Stderr, "start daemon failed: %v\n", err)
			os.Exit(1)
		}
	case "serve":
		svc := daemon.New(*cfgPath, cfg, logger)
		if err := svc.Run(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "service failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", sub)
		os.Exit(2)
	}
}

func buildLogger(path string) (*log.Logger, func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, func() {}, err
	}
	return log.New(f, "nesd ", log.LstdFlags|log.Lmicroseconds), func() { _ = f.Close() }, nil
}
