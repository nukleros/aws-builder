/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nukleros/aws-builder/pkg/client"
	"github.com/nukleros/aws-builder/pkg/config"
	"github.com/nukleros/aws-builder/pkg/rds"
)

// deleteCmd represents the delete command.
var deleteCmd = &cobra.Command{
	Use:   "delete <resource stack> <inventory file>",
	Short: "Remove an AWS resource stack",
	Long:  `Remove an AWS resource stack.`,
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

		// capture messages as resources are deleted and return to user
		go func() {
			for msg := range *resourceClient.MessageChan {
				fmt.Println(msg)
			}
		}()

		// call requested resource stack deletion
		switch args[0] {
		case "rds":
			// create client and config for resource deletion
			rdsClient, rdsInventory, err := rds.InitDelete(resourceClient, args[1])
			if err != nil {
				return fmt.Errorf("failed to initialize resource client and config: %w", err)
			}

			fmt.Println("deleting RDS instance resource stack...")

			// delete resources
			if err := rdsClient.DeleteRdsResourceStack(rdsInventory); err != nil {
				return fmt.Errorf("failed to remove RDS resource stack: %w", err)
			}
			fmt.Println("RDS resource stack deleted")
		default:
			return errors.New("unrecognized resource stack")
		}

		// allow 3 seconds for final inventory updates to be made so file can be
		// properly deleted
		time.Sleep(time.Second * 3)

		// remove inventory file from filesystem
		if err := os.Remove(args[1]); err != nil {
			return fmt.Errorf("failed to remove eks cluster inventory file: %w", err)
		}

		fmt.Printf("Inventory file '%s' deleted\n", args[1])

		fmt.Println("AWS resources deleted")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
