package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
)

func newCompletionCommand() *cobra.Command {
	completionCmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactArgs(1),
		Short:     "Generates completion scripts",
		Long:      "Generates completion scripts for various shells",
		Example: "  " + os.Args[0] + " completion bash > completion.sh\n" +
			"  " + "source completion.sh",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout, _, err := feedback.DirectStreams()
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			completionNoDesc, _ := cmd.Flags().GetBool("no-descriptions")

			shell := args[0]
			switch shell {
			case "bash":
				return cmd.Root().GenBashCompletionV2(stdout, !completionNoDesc)
			case "zsh":
				if completionNoDesc {
					return cmd.Root().GenZshCompletionNoDesc(stdout)
				}
				return cmd.Root().GenZshCompletion(stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(stdout, !completionNoDesc)
			case "powershell":
				return cmd.Root().GenPowerShellCompletion(stdout)
			default:
				return cmd.Usage() // Handle invalid shell argument
			}
		},
	}

	completionCmd.Flags().Bool("no-descriptions", false, "Disable completion description for shells that support it")

	return completionCmd
}
