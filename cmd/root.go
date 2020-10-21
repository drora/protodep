// Copyright © 2017 stormcat24 <stormcat24@stormcat.io>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "protodep",
	Short: "Manage vendor for Protocol Buffer IDL file (.proto)",
	Long: `
	protodep allows us to define a TOML configuration file with all the dependencies a service needs.
	This can be 3rd party proto modules or another internal service or message it needs.
	Protodep will parse the TOML file and download all the dependencies to configured sub directory.
	
	Recommendation: commit protodep.toml and protodep.lock files only to your source control.
	The protodep directory which contains the downloaded assets should not be committed into source control (just as you wouldn’t normally commit node_modules).
	Only exception is in cases where you would like to extend or override specific imported assets, in this case, those extended assets should be commited as well.`,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		color.Red(err.Error())
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {

}
