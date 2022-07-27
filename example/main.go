package main

import (
	"github.com/alob-mtc/migrator/example/cmd"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "migrator",
		Short: "A brief description of your application",
	}

	rootCmd.AddCommand(
		cmd.Migrations(),
	)

	cobra.CheckErr(rootCmd.Execute())
}
