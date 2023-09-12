/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const supportedResourceStacks = `
Supported resource stacks:
* rds (Relational Database Service)
* s3 (Simple Storage Service)`

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "aws-builder",
	Short: "Manage AWS resource stacks",
	Long: fmt.Sprintf(`Manage AWS resource stacks.  This tool allows you to manage all the resources
needed for particular managed services that serve applications.
%s`, supportedResourceStacks),
}

var (
	awsConfigProfile string
	awsRegion        string
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringVarP(&awsConfigProfile, "aws-config-profile", "p", "default",
		"The AWS config profile to draw credentials from when provisioning resources")
	rootCmd.PersistentFlags().StringVarP(&awsRegion, "aws-region", "r", "",
		"AWS region to create resources in - if defined will override region in config profile")
}
