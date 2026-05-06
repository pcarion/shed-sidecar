package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	version = "dev"
	address = "127.0.0.1:50051"
)

func main() {
	root := &cobra.Command{
		Use:           "shed-sidecar",
		Short:         "Interact with the shed sidecar daemon",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&address, "address", address, "shed-sidecard gRPC address")
	root.AddCommand(statusCommand(), passwordCommand(), networkCommand(), paramCommand(), postgresCommand(), confCommand(), versionCommand())

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
	cmd.AddCommand(passwordGetCommand(), passwordReadCommand(), passwordListCommand())
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

func passwordReadCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "read <service name> <name>",
		Short: "Read a stored password without creating it",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).PasswordRead(ctx, &sidecarv1.PasswordReadRequest{
				ServiceName: args[0],
				Name:        args[1],
			})
			if err != nil {
				return fmt.Errorf("password read RPC: %w", err)
			}
			if !resp.GetIsOk() {
				return fmt.Errorf("password %q for service %q was not found", args[1], args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.GetPassword())
			return nil
		},
	}
}

func passwordListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored passwords",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).PasswordList(ctx, &sidecarv1.PasswordListRequest{})
			if err != nil {
				return fmt.Errorf("password list RPC: %w", err)
			}
			printPasswordList(cmd.OutOrStdout(), resp)
			return nil
		},
	}
}

func networkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage sidecar network ports",
	}
	portCmd := &cobra.Command{
		Use:   "port",
		Short: "Manage sidecar network ports",
	}
	portCmd.AddCommand(networkPortGetCommand(), networkListCommand())
	cmd.AddCommand(portCmd, networkListCommand())
	return cmd
}

func networkPortGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <service name> <name>",
		Short: "Get or allocate an idempotent network port",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).NetworkPortGet(ctx, &sidecarv1.NetworkPortGetRequest{
				ServiceName: args[0],
				Name:        args[1],
			})
			if err != nil {
				return fmt.Errorf("network port get RPC: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.GetPort())
			return nil
		},
	}
}

func networkListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored network ports",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).NetworkList(ctx, &sidecarv1.NetworkListRequest{})
			if err != nil {
				return fmt.Errorf("network list RPC: %w", err)
			}
			printNetworkList(cmd.OutOrStdout(), resp)
			return nil
		},
	}
}

func paramCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "param",
		Short: "Manage sidecar parameters",
	}
	cmd.AddCommand(paramSetCommand(), paramGetCommand(), paramListCommand())
	return cmd
}

func paramSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <service name> <name> <value>",
		Short: "Set a stored parameter",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			_, err = sidecarv1.NewSidecarClient(conn).ParamSet(ctx, &sidecarv1.ParamSetRequest{
				ServiceName: args[0],
				Name:        args[1],
				Value:       args[2],
			})
			if err != nil {
				return fmt.Errorf("param set RPC: %w", err)
			}
			return nil
		},
	}
}

func paramGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <service name> <name>",
		Short: "Get a stored parameter",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).ParamGet(ctx, &sidecarv1.ParamGetRequest{
				ServiceName: args[0],
				Name:        args[1],
			})
			if err != nil {
				return fmt.Errorf("param get RPC: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.GetValue())
			return nil
		},
	}
}

func paramListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).ParamList(ctx, &sidecarv1.ParamListRequest{})
			if err != nil {
				return fmt.Errorf("param list RPC: %w", err)
			}
			printParamList(cmd.OutOrStdout(), resp)
			return nil
		},
	}
}

func postgresCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "postgres",
		Short: "Manage PostgreSQL configuration",
	}
	pgHbaCmd := &cobra.Command{
		Use:   "pg-hba",
		Short: "Manage PostgreSQL pg_hba.conf rules",
	}
	pgHbaCmd.AddCommand(pgHbaConfigureCommand())
	cmd.AddCommand(pgHbaCmd)
	return cmd
}

func pgHbaConfigureCommand() *cobra.Command {
	var clientAddress string
	var options string
	cmd := &cobra.Command{
		Use:   "configure <file path> <type> <database> <users> <method>",
		Short: "Configure a pg_hba.conf rule",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			hbaType, err := parsePgHbaType(args[1])
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

			req := &sidecarv1.ConfigurePgHbaConfRequest{
				FilePath: args[0],
				Type:     hbaType,
				Database: args[2],
				Users:    splitCSV(args[3]),
				Method:   args[4],
			}
			if clientAddress != "" {
				req.Address = &clientAddress
			}
			if options != "" {
				req.Options = &options
			}

			resp, err := sidecarv1.NewSidecarClient(conn).ConfigurePgHbaConf(ctx, req)
			if err != nil {
				return fmt.Errorf("configure pg_hba.conf RPC: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "valid=%t new=%t\n", resp.GetIsValid(), resp.GetIsNew())
			return nil
		},
	}
	cmd.Flags().StringVar(&clientAddress, "client-address", "", "pg_hba address column for host rules")
	cmd.Flags().StringVar(&options, "options", "", "pg_hba options columns")
	return cmd
}

func confCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conf",
		Short: "Manage key/value configuration files",
	}
	cmd.AddCommand(confSetCommand(), confGetCommand())
	return cmd
}

func confSetCommand() *cobra.Command {
	valueTypeName := "string"
	cmd := &cobra.Command{
		Use:   "set <file path> <space|equal|colon> <key=value> ...",
		Short: "Set missing keys in a key/value configuration file",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			confType, err := parseKeyValueConfType(args[1])
			if err != nil {
				return err
			}
			valueType, err := parseKeyValueValueType(valueTypeName)
			if err != nil {
				return err
			}
			entries := make([]*sidecarv1.KeyValueEntry, 0, len(args)-2)
			for _, arg := range args[2:] {
				key, value, err := splitKeyValueArg(arg)
				if err != nil {
					return err
				}
				entries = append(entries, &sidecarv1.KeyValueEntry{
					Key:   key,
					Value: value,
					Type:  &valueType,
				})
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			conn, err := dial()
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := sidecarv1.NewSidecarClient(conn).ConfigureKeyValueConf(ctx, &sidecarv1.ConfigureKeyValueConfRequest{
				FilePath: args[0],
				Type:     confType,
				Entries:  entries,
			})
			if err != nil {
				return fmt.Errorf("configure key/value conf RPC: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "valid=%t new=%t\n", resp.GetIsValid(), resp.GetIsNew())
			return nil
		},
	}
	cmd.Flags().StringVar(&valueTypeName, "value-type", valueTypeName, "value type: string or number")
	return cmd
}

func confGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <file path> <space|equal|colon> <key>",
		Short: "Get a key from a key/value configuration file",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			confType, err := parseKeyValueConfType(args[1])
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

			resp, err := sidecarv1.NewSidecarClient(conn).ConfigureGetKeyValue(ctx, &sidecarv1.ConfigureGetKeyValueRequest{
				FilePath: args[0],
				Type:     confType,
				Key:      args[2],
			})
			if err != nil {
				return fmt.Errorf("get key/value conf RPC: %w", err)
			}
			if !resp.GetIsValid() {
				return fmt.Errorf("key %q was not found", args[2])
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.GetValue())
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
	case "symbol", "symbols", "@", "password_type_symbol":
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

func parsePgHbaType(value string) (sidecarv1.PgHbaType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "local", "pg_hba_type_local":
		return sidecarv1.PgHbaType_PG_HBA_TYPE_LOCAL, nil
	case "host", "pg_hba_type_host":
		return sidecarv1.PgHbaType_PG_HBA_TYPE_HOST, nil
	default:
		return sidecarv1.PgHbaType_PG_HBA_TYPE_UNSPECIFIED, fmt.Errorf("unknown pg_hba type %q", value)
	}
}

func parseKeyValueConfType(value string) (sidecarv1.KeyValueConfType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "space", "key-value", "key_value", "key_value_conf_type_space":
		return sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_SPACE, nil
	case "equal", "equals", "=", "key_value_conf_type_equal":
		return sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL, nil
	case "colon", ":", "key_value_conf_type_colon":
		return sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_COLON, nil
	default:
		return sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_UNSPECIFIED, fmt.Errorf("unknown key/value conf type %q", value)
	}
}

func parseKeyValueValueType(value string) (sidecarv1.KeyValueValueType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "string", "str", "key_value_value_type_string":
		return sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_STRING, nil
	case "number", "num", "key_value_value_type_number":
		return sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER, nil
	default:
		return sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_UNSPECIFIED, fmt.Errorf("unknown key/value value type %q", value)
	}
}

func splitKeyValueArg(value string) (string, string, error) {
	key, val, ok := strings.Cut(value, "=")
	if !ok {
		return "", "", fmt.Errorf("invalid key/value argument %q: expected key=value", value)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", fmt.Errorf("invalid key/value argument %q: key is empty", value)
	}
	return key, val, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func printPasswordList(out io.Writer, resp *sidecarv1.PasswordListResponse) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tNAME\tPASSWORD")
	for _, service := range resp.GetServices() {
		for _, password := range service.GetPasswords() {
			fmt.Fprintf(w, "%s\t%s\t%s\n", service.GetServiceName(), password.GetName(), password.GetPassword())
		}
	}
	_ = w.Flush()
}

func printNetworkList(out io.Writer, resp *sidecarv1.NetworkListResponse) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tNAME\tPORT")
	for _, network := range resp.GetNetworks() {
		fmt.Fprintf(w, "%s\t%s\t%d\n", network.GetServiceName(), network.GetName(), network.GetPort())
	}
	_ = w.Flush()
}

func printParamList(out io.Writer, resp *sidecarv1.ParamListResponse) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tNAME\tVALUE")
	for _, service := range resp.GetServices() {
		for _, param := range service.GetParams() {
			fmt.Fprintf(w, "%s\t%s\t%s\n", service.GetServiceName(), param.GetName(), param.GetValue())
		}
	}
	_ = w.Flush()
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
