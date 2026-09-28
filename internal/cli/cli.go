// Package cli defines the Cobra command tree.
package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Sabissimo/http-proxy/internal/config"
	"github.com/Sabissimo/http-proxy/internal/logging"
	"github.com/Sabissimo/http-proxy/internal/server"
	"github.com/Sabissimo/http-proxy/internal/winsvc"
)

// Execute runs the command line. Errors are logged here; the caller only sets
// the exit code.
func Execute() error {
	var logCloser io.Closer = io.NopCloser(nil)
	root := &cobra.Command{
		Use:           "httpproxy",
		Short:         "Reverse proxy forwarding every request to TARGET",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			logCfg, err := config.LoadLog()
			if err != nil {
				return err
			}
			closer, err := logging.Setup(winsvc.Interactive(), logCfg.Dir, logCfg.KeepDays)
			if err != nil {
				return err
			}
			logCloser = closer
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return runServe(cmd.Context()) },
	}
	root.AddCommand(serveCmd(), serviceCmd())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := root.ExecuteContext(ctx)
	if err != nil {
		slog.Error("fatal", "err", err)
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	_ = logCloser.Close()
	return err
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the proxy in the foreground",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return runServe(cmd.Context()) },
	}
}

func serviceCmd() *cobra.Command {
	var flags winsvc.Options
	cmd := &cobra.Command{
		Use:   "service <" + strings.Join(winsvc.Actions, "|") + ">",
		Short: "Control the Windows service",
		Long: "Control the Windows service. The name, display name and description come from the\n" +
			"flags, else SERVICE_NAME / SERVICE_DISPLAY_NAME / SERVICE_DESCRIPTION in .env, else the\n" +
			"defaults. Pass the same --name to start, stop and uninstall as to install.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: winsvc.Actions,
		RunE: func(_ *cobra.Command, args []string) error {
			if !slices.Contains(winsvc.Actions, args[0]) {
				return fmt.Errorf("unknown service action %q (expected %s)", args[0], strings.Join(winsvc.Actions, "|"))
			}
			opts, err := serviceOptions(flags)
			if err != nil {
				return err
			}
			// The service manager supplies its own stop signal, so the run
			// function gets the service's context, not the console one.
			return winsvc.Control(args[0], opts, runServe)
		},
	}
	cmd.Flags().StringVar(&flags.Name, winsvc.NameFlag, "", "service name (default "+winsvc.DefaultName+")")
	cmd.Flags().StringVar(&flags.DisplayName, "display-name", "", "name shown in services.msc (default "+winsvc.DefaultDisplayName+")")
	cmd.Flags().StringVar(&flags.Description, "description", "", "service description")
	return cmd
}

// serviceOptions layers the flags over .env over the defaults.
func serviceOptions(flags winsvc.Options) (winsvc.Options, error) {
	env, err := config.LoadService()
	if err != nil {
		return winsvc.Options{}, err
	}
	opts := winsvc.Options{Name: env.Name, DisplayName: env.DisplayName, Description: env.Description}
	if flags.Name != "" {
		opts.Name = flags.Name
	}
	if flags.DisplayName != "" {
		opts.DisplayName = flags.DisplayName
	}
	if flags.Description != "" {
		opts.Description = flags.Description
	}
	return opts.WithDefaults(), nil
}

func runServe(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return server.Serve(ctx, cfg)
}
