package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strings"
)

var storageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List storage",
	Long: `LIST show all available storage within a project and environment. For example:

	qovery storage list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowStorageList(ProjectName, BranchName)
	},
}

func init() {
	storageListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	storageListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")

	storageCmd.AddCommand(storageListCmd)
}

func ShowStorageList(projectName string, branchName string) {
	output := []string{
		"name | status | type | version | endpoint | port | username | password | application",
	}

	services := api.ListStorage(api.GetProjectByName(projectName).Id, branchName)

	if services.Results == nil || len(services.Results) == 0 {
		fmt.Println(columnize.SimpleFormat(output))
		return
	}

	for _, a := range services.Results {
		applicationName := "none"

		if a.Application != nil {
			applicationName = a.Application.Name
		}
		output = append(output, strings.Join([]string{
			a.Name,
			a.Status.CodeMessage,
			a.Type,
			a.Version,
			a.FQDN,
			intPointerValue(a.Port),
			a.Username,
			a.Password,
			applicationName,
		}, " | "))
	}

	fmt.Println(columnize.SimpleFormat(output))
}