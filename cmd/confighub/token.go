package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/ruichen/config-hub/internal/secret"
	"github.com/spf13/cobra"
)

func newTokenCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "token", Short: "Manage ConfigHub bearer tokens", Example: "confighub token create --label macbook --scope pull:macbook"}
	cmd.AddCommand(newTokenCreateCommand(), newTokenListCommand(), newTokenRevokeCommand())
	return cmd
}

func newTokenCreateCommand() *cobra.Command {
	var label, scope, rootFlag string
	cmd := &cobra.Command{
		Use:     "create --label <label> --scope <scope> [--root <dir>]",
		Short:   "Create a bearer token and print it once",
		Example: "confighub token create --label macbook --scope pull:macbook --root examples",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("token create does not accept positional arguments")}
			}
			if label == "" || scope == "" {
				return &exitError{code: exitUsage, err: fmt.Errorf("--label and --scope are required")}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			plaintext, _, err := secret.Create(root, label, scope)
			if err != nil {
				if errors.Is(err, secret.ErrInvalidScope) {
					return &exitError{code: exitValidation, err: err}
				}
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), plaintext)
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "operator-visible token label")
	cmd.Flags().StringVar(&scope, "scope", "", "token scope: admin, pull:<profile>, or read:<template>")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	return cmd
}

func newTokenListCommand() *cobra.Command {
	var rootFlag string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "list [--root <dir>] [--json]",
		Short:   "List token metadata",
		Example: "confighub token list --root examples --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("token list does not accept positional arguments")}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			tokens, err := secret.List(root)
			if err != nil {
				return err
			}
			if jsonOut {
				type publicToken struct {
					ID        string `json:"id"`
					Label     string `json:"label"`
					Scope     string `json:"scope"`
					CreatedAt string `json:"createdAt"`
				}
				out := make([]publicToken, 0, len(tokens))
				for _, tok := range tokens {
					out = append(out, publicToken{ID: tok.ID, Label: tok.Label, Scope: tok.Scope, CreatedAt: tok.CreatedAt})
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tLABEL\tSCOPE\tCREATED AT")
			for _, tok := range tokens {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", tok.ID, tok.Label, tok.Scope, tok.CreatedAt)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON token metadata")
	return cmd
}

func newTokenRevokeCommand() *cobra.Command {
	var rootFlag string
	cmd := &cobra.Command{
		Use:     "revoke <id> [--root <dir>]",
		Short:   "Revoke a bearer token",
		Example: "confighub token revoke cfh_abcd --root examples",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &exitError{code: exitUsage, err: fmt.Errorf("token revoke requires exactly one token id")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			if err := secret.Revoke(root, args[0]); err != nil {
				if errors.Is(err, secret.ErrTokenNotFound) {
					return &exitError{code: exitValidation, err: err}
				}
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	return cmd
}
