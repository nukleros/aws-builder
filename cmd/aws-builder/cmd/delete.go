/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/nukleros/aws-builder/pkg/client"
	"github.com/nukleros/aws-builder/pkg/config"
	"github.com/nukleros/aws-builder/pkg/eks"
	"github.com/nukleros/aws-builder/pkg/rds"
	"github.com/nukleros/aws-builder/pkg/s3"
)

// deleteCmd represents the delete command.
var deleteCmd = &cobra.Command{
	Use:   "delete <resource stack> <inventory file>",
	Short: "Remove an AWS resource stack",
	Long: fmt.Sprintf(`Remove an AWS resource stack.
%s`, supportedResourceStacks),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("deleting AWS resource stack...")

		// ensure resource stack argument provided
		if len(args) < 2 {
			return fmt.Errorf("missing arguments")
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
		var deleteWait sync.WaitGroup

		// capture messages as resources are deleted and return to user
		deleteWait.Add(1)
		go func() {
			defer deleteWait.Done()
			for msg := range *resourceClient.MessageChan {
				fmt.Println(msg)
			}
		}()

		// call requested resource stack deletion
		switch args[0] {
		case "eks":
			// create client and config for resource deletion
			invChan := make(chan eks.EksInventory)
			eksClient, eksInventory, err := eks.InitDelete(
				resourceClient,
				args[1],
				&invChan,
				&deleteWait,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize EKS resource client and inventory: %w", err)
			}

			// delete resources
			if err := eksClient.DeleteEksResourceStack(eksInventory); err != nil {
				return fmt.Errorf("failed to remove EKS resource stack: %w", err)
			}
			close(invChan)
		case "rds":
			// create client and config for resource deletion
			invChan := make(chan rds.RdsInventory)
			rdsClient, rdsInventory, err := rds.InitDelete(
				resourceClient,
				args[1],
				&invChan,
				&deleteWait,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize RDS resource client and inventory: %w", err)
			}

			// delete resources
			if err := rdsClient.DeleteRdsResourceStack(rdsInventory); err != nil {
				return fmt.Errorf("failed to remove RDS resource stack: %w", err)
			}
			close(invChan)
		case "s3":
			// create client and config for resource deletion
			invChan := make(chan s3.S3Inventory)
			s3Client, s3Inventory, err := s3.InitDelete(
				resourceClient,
				args[1],
				&invChan,
				&deleteWait,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize S3 resource client and inventory: %w", err)
			}

			// delete resources
			if err := s3Client.DeleteS3ResourceStack(s3Inventory); err != nil {
				return fmt.Errorf("failed to remove S3 resource stack: %w", err)
			}
			close(invChan)
		default:
			return errors.New("unrecognized resource stack")
		}

		close(*resourceClient.MessageChan)

		// wait until all inventory and message goroutines have completed
		deleteWait.Wait()
		fmt.Println("AWS resources deleted")

		// remove inventory file from filesystem
		if err := os.Remove(args[1]); err != nil {
			return fmt.Errorf("failed to remove eks cluster inventory file: %w", err)
		}
		fmt.Printf("Inventory file '%s' deleted\n", args[1])

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
