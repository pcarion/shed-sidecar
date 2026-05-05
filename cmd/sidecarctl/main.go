package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
	root.AddCommand(statusCommand(), passwordCommand(), versionCommand())

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

			conn, err := dial()
			if err != nil {
				return err
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

func passwordCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "password",
		Short: "Manage sidecar passwords",
	}
	cmd.AddCommand(passwordGetCommand())
	return cmd
}

func passwordGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <service name> <name> <length> <type>",
		Short: "Get or create an idempotent password",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			length64, err := strconv.ParseInt(args[2], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid length %q: %w", args[2], err)
			}
			passwordType, err := parsePasswordType(args[3])
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).PasswordGet(ctx, &sidecarv1.PasswordGetRequest{
				ServiceName: args[0],
				Name:        args[1],
				Length:      int32(length64),
				Type:        passwordType,
			})
			if err != nil {
				return fmt.Errorf("password get RPC: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.GetPassword())
			return nil
		},
	}
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

func dial() (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}
	return conn, nil
}

func parsePasswordType(value string) (sidecarv1.PasswordType, error) {
	trimmed := strings.TrimSpace(value)
	switch trimmed {
	case "a":
		return sidecarv1.PasswordType_PASSWORD_TYPE_LOWERCASE, nil
	case "A":
		return sidecarv1.PasswordType_PASSWORD_TYPE_UPPERCASE, nil
	case "h":
		return sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER, nil
	case "H":
		return sidecarv1.PasswordType_PASSWORD_TYPE_HEX_UPPER, nil
	}

	switch strings.ToLower(trimmed) {
	case "lowercase", "lower", "password_type_lowercase":
		return sidecarv1.PasswordType_PASSWORD_TYPE_LOWERCASE, nil
	case "uppercase", "upper", "password_type_uppercase":
		return sidecarv1.PasswordType_PASSWORD_TYPE_UPPERCASE, nil
	case "digit", "digits", "number", "numbers", "1", "password_type_digit":
		return sidecarv1.PasswordType_PASSWORD_TYPE_DIGIT, nil
	case "symbol", "symbols", "#", "password_type_symbol":
		return sidecarv1.PasswordType_PASSWORD_TYPE_SYMBOL, nil
	case "hex-lower", "hex_lower", "hexlower", "password_type_hex_lower":
		return sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER, nil
	case "hex-upper", "hex_upper", "hexupper", "password_type_hex_upper":
		return sidecarv1.PasswordType_PASSWORD_TYPE_HEX_UPPER, nil
	case "uuid-v7", "uuid_v7", "uuidv7", "u7", "password_type_uuid_v7":
		return sidecarv1.PasswordType_PASSWORD_TYPE_UUID_V7, nil
	default:
		return sidecarv1.PasswordType_PASSWORD_TYPE_UNSPECIFIED, fmt.Errorf("unknown password type %q", value)
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
