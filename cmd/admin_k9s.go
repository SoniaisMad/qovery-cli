package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"qovery-cli/io"
	"qovery-cli/utils"
)

var k9sCmd = &cobra.Command{
	Use: "k9s",
	Short: "Launch k9s with a cluster ID",
	Run: func(cmd *cobra.Command, args []string) {
		launchK9s(args)
	},
}

func init(){
	adminCmd.AddCommand(k9sCmd)
}

func launchK9s(args []string){
	checkEnv()

	if len(args) == 0  {
		log.Error("You must enter a cluster ID.")
		return
	}

	vars := io.GetVarsByClusterId(args[0])
	if len(vars) == 0 {
		return
	}

	for _, variable := range vars {
		os.Setenv(variable.Key, variable.Value)
	}
	 utils.GenerateExportEnvVarsScript(vars, args[0])

	log.Info("Launching k9s.")
	cmd := exec.Command("k9s")
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Error("Can't launch k9s : " + err.Error())
	}

	utils.DeleteFolder(os.Getenv("KUBECONFIG")[0:len(os.Getenv("KUBECONFIG"))-len("kubeconfig")])
}

func checkEnv(){
	var isError bool
	if os.Getenv("VAULT_ADDR") == "" {
		log.Error("You must set vault address env variable (VAULT_ADDR).")
		isError = true
	}

	if os.Getenv("VAULT_TOKEN") == "" {
		log.Error("You must set vault token env variable (VAULT_TOKEN).")
		isError = true
	}

	if isError {
		os.Exit(1)
	}
}
