/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nukleros/aws-builder/pkg/client"
	"github.com/nukleros/aws-builder/pkg/config"
	"github.com/nukleros/aws-builder/pkg/rds"
)

var createInventoryFile string

// createCmd represents the create command.
var createCmd = &cobra.Command{
	Use:   "create <resource stack> <config file>",
	Short: "Provision an AWS resource stack",
	Long: `Provision an AWS resource stack.
Supported resource stacks:
* rds (Relational Database Service)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// ensure resource stack argument provided
		if len(args) < 2 {
			return fmt.Errorf("missing arguments")
		}

		// load AWS config
		awsConfig, err := config.LoadAwsConfig(awsConfigProfile, awsRegion)
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w", err)
		}

		// create resource client
		resourceClient := client.CreateResourceClient(awsConfig)

		// capture messages as resources are created and return to user
		go func() {
			for msg := range *resourceClient.MessageChan {
				fmt.Println(msg)
			}
		}()

		// call requested resource stack creation
		switch args[0] {
		case "rds":
			// create client and config for resource creation
			rdsClient, rdsConfig, err := rds.InitCreate(resourceClient, args[1], createInventoryFile)
			if err != nil {
				return fmt.Errorf("failed to initialize resource client and config: %w", err)
			}

			fmt.Println("creating RDS instance resource stack...")

			// create resources
			if err := rdsClient.CreateRdsResourceStack(rdsConfig); err != nil {
				return fmt.Errorf("failed to create RDS resource stack: %w", err)
			}
			fmt.Println("RDS resource stack created")
		default:
			return errors.New("unrecognized resource stack")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(
		&createInventoryFile, "inventory-file", "i", "aws-inventory.json",
		"File to write AWS resource inventory to",
	)
}
