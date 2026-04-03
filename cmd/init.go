package cmd

import (
	"fmt"

	"github.com/mateusduraes/ramo/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new ramo.json configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := getWorkingDir()
		if err != nil {
			return err
		}

		if err := config.CreateDefault(dir); err != nil {
			return err
		}

		fmt.Println("Created ramo.json with default configuration")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
