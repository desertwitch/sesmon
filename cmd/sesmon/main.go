/*
sesmon - SES monitoring and alerting daemon
*/
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Version is the program version as filled in by the Makefile.
var Version string

// newRootCmd returns the primary [cobra.Command] pointer for the program.
func newRootCmd(ctx context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "sesmon",
		Short:             "SES monitoring and alerting daemon",
		Version:           Version,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	monitorCmd := newMonitorCmd(ctx)
	checkCmd := newCheckCmd()
	testCmd := newTestCmd()

	rootCmd.AddCommand(monitorCmd, checkCmd, testCmd)

	return rootCmd
}

// newMonitorCmd returns the "monitor" [cobra.Command] pointer for the program.
func newMonitorCmd(ctx context.Context) *cobra.Command {
	monitorCmd := &cobra.Command{
		Use:   "monitor <config.yaml>",
		Short: "Monitor target SES-capable devices using a configuration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			yamlConfig, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failure reading configuration file: %w", err)
			}

			prog, err := NewProgram(yamlConfig, nil, nil, nil, os.Stderr)
			if err != nil {
				return fmt.Errorf("failure establishing program: %w", err)
			}

			prog.Start(ctx)
			<-prog.Done()

			return nil
		},
	}

	return monitorCmd
}

// newMonitorCmd returns the "check" [cobra.Command] pointer for the program.
func newCheckCmd() *cobra.Command {
	checkCmd := &cobra.Command{
		Use:   "check <config.yaml>",
		Short: "Check if a configuration file is syntactically parseable (YAML)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			yamlConfig, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failure reading configuration file: %w", err)
			}

			decoder := yaml.NewDecoder(bytes.NewReader(yamlConfig))
			decoder.KnownFields(true)

			var config ConfigYAML
			if err := decoder.Decode(&config); err != nil {
				return fmt.Errorf("failure parsing YAML: %w", err)
			}

			return nil
		},
	}

	return checkCmd
}

// // newMonitorCmd returns the "test" [cobra.Command] pointer for the program.
func newTestCmd() *cobra.Command {
	testCmd := &cobra.Command{
		Use:   "test <config.yaml>",
		Short: "Test if enabled devices of a configuration file can be resolved",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			yamlConfig, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failure reading configuration file: %w", err)
			}

			_, err = NewProgram(yamlConfig, nil, nil, nil, os.Stderr)
			if err != nil {
				return fmt.Errorf("failure establishing program: %w", err)
			}

			return nil
		},
	}

	return testCmd
}

func main() {
	var exitCode int
	defer func() {
		os.Exit(exitCode)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer recoverGoPanic("signals", nil)
		<-sigs
		cancel()
	}()

	rootCmd := newRootCmd(ctx)
	if err := rootCmd.Execute(); err != nil {
		exitCode = 1
	}
}
