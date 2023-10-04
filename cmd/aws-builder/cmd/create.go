/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/nukleros/aws-builder/pkg/client"
	"github.com/nukleros/aws-builder/pkg/config"
	"github.com/nukleros/aws-builder/pkg/rds"
	"github.com/nukleros/aws-builder/pkg/s3"
)

var createInventoryFile string

// createCmd represents the create command.
var createCmd = &cobra.Command{
	Use:   "create <resource stack> <config file>",
	Short: "Provision an AWS resource stack",
	Long: fmt.Sprintf(`Provision an AWS resource stack.
%s`, supportedResourceStacks),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("creating AWS resource stack...")

		// ensure resource stack argument provided
		if len(args) < 2 {
			return fmt.Errorf("missing arguments")
		}

		// create default inventory filename if not provided
		if createInventoryFile == "" {
			createInventoryFile = fmt.Sprintf("%s-inventory.json", args[0])
		}

		// load AWS config
		awsConfig, err := config.LoadAWSConfig(awsConfigProfile, awsRegion, awsRoleArn, awsSerialNumber)
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w", err)
		}

		// create resource client
		resourceClient := client.CreateResourceClient(awsConfig)

		// use a wait group to ensure messages and inventory are processed
		// before quitting
		var createWait sync.WaitGroup

		// capture messages as resources are created and return to user
		createWait.Add(1)
		go func() {
			defer createWait.Done()
			for msg := range *resourceClient.MessageChan {
				fmt.Println(msg)
			}
		}()

		// call requested resource stack creation
		switch args[0] {
		case "rds":
			// create client and config for resource creation
			invChan := make(chan rds.RdsInventory)
			rdsClient, rdsConfig, err := rds.InitCreate(
				resourceClient,
				args[1],
				createInventoryFile,
				&invChan,
				&createWait,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize RDS resource client and config: %w", err)
			}

			// create resources
			if err := rdsClient.CreateRdsResourceStack(rdsConfig); err != nil {
				return fmt.Errorf("failed to create RDS resource stack: %w", err)
			}
			close(invChan)
		case "s3":
			// create client and config for resource creation
			invChan := make(chan s3.S3Inventory)
			s3Client, s3Config, err := s3.InitCreate(
				resourceClient,
				args[1],
				createInventoryFile,
				&invChan,
				&createWait,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize S3 resource client and config: %w", err)
			}

			// create resources
			if err := s3Client.CreateS3ResourceStack(s3Config); err != nil {
				return fmt.Errorf("failed to create S3 resource stack: %w", err)
			}
			close(invChan)
		default:
			return errors.New("unrecognized resource stack")
		}

		close(*resourceClient.MessageChan)

		// wait until all inventory and message goroutines have completed
		createWait.Wait()
		fmt.Println("AWS resource stack created")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(
		&createInventoryFile, "inventory-file", "i", "",
		"File to write AWS resource inventory to",
	)
}
