package cmd

import (
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/stormcat24/protodep/session"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login to github with a personal-access-token",
	RunE: func(cdm *cobra.Command, args []string) error {
		homeDir, err := homedir.Dir()
		if err != nil {
			return err
		}

		var sessionService = session.New(&session.Config{
			HomeDir: homeDir,
		})
		return sessionService.Login()
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "logout from github",
	RunE: func(cdm *cobra.Command, args []string) error {
		homeDir, err := homedir.Dir()
		if err != nil {
			return err
		}

		var sessionService = session.New(&session.Config{
			HomeDir: homeDir,
		})
		return sessionService.Logout()
	},
}
