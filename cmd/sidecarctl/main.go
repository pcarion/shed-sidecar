package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	version = "dev"
	address = "127.0.0.1:8443"
)

func main() {
	root := &cobra.Command{
		Use:           "sidecarctl",
		Short:         "Interact with the shed sidecar daemon",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&address, "address", address, "sidecard gRPC address")
	root.AddCommand(statusCommand(), versionCommand())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func statusCommand() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "status <service1> <service2> ...",
		Short: "Print systemd service status",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, services []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("connect to %s: %w", address, err)
			}
			defer conn.Close()

			req := &sidecarv1.ServiceStatusRequest{Services: services, IncludeRaw: verbose}

			resp, err := sidecarv1.NewSidecarClient(conn).ServiceStatus(ctx, req)
			if err != nil {
				return fmt.Errorf("service status RPC: %w", err)
			}

			if verbose {
				printVerbose(resp)
				return nil
			}
			printTable(resp)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print raw systemctl status output")
	return cmd
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	}
}

func printTable(resp *sidecarv1.ServiceStatusResponse) {
	fmt.Printf("%-2s %-32s %-12s %-16s %s\n", "", "SERVICE", "ACTIVE", "SUB", "DESCRIPTION")
	for _, status := range resp.GetStatuses() {
		fmt.Printf("%-2s %-32s %-12s %-16s %s\n",
			symbol(status.GetActiveState()),
			status.GetName(),
			status.GetActiveState(),
			status.GetSubState(),
			status.GetDescription(),
		)
	}
}

func printVerbose(resp *sidecarv1.ServiceStatusResponse) {
	for i, status := range resp.GetStatuses() {
		if i > 0 {
			fmt.Println()
		}
		if raw := status.GetRaw(); raw != "" {
			fmt.Print(raw)
			if !strings.HasSuffix(raw, "\n") {
				fmt.Println()
			}
			continue
		}
		fmt.Printf("%s %s %s %s %s\n", symbol(status.GetActiveState()), status.GetName(), status.GetActiveState(), status.GetSubState(), status.GetDescription())
	}
}

func symbol(activeState string) string {
	switch activeState {
	case "active":
		return "✓"
	case "inactive", "failed":
		return "✗"
	default:
		return "○"
	}
}
