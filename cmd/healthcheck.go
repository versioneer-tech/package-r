package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	command := &cobra.Command{
		Use:   "healthcheck",
		Short: "Check the local HTTP health endpoint",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return checkHealth(command.Context(), getParam(command.Flags(), "address"), getParam(command.Flags(), "port"))
		},
	}
	command.Flags().StringP("address", "a", "127.0.0.1", "server address")
	command.Flags().StringP("port", "p", "8888", "server port")
	rootCmd.AddCommand(command)
}

func checkHealth(ctx context.Context, address, port string) error {
	address = strings.Trim(address, "[]")
	if address == "" || address == "0.0.0.0" || address == "::" {
		address = "127.0.0.1"
	}
	if port == "" {
		port = "8888"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(address, port)+"/health", http.NoBody)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}
	// A local probe must not use a proxy or follow a redirect to another service.
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("check health: %w", err)
	}
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close health response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}
