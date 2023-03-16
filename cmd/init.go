package cmd

func init() {
	RootCmd.AddCommand(upCmd, versionCmd, loginCmd, logoutCmd)
	initDepCmd()
}
