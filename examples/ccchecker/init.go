/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"

	"github.com/hyperledger/fabric/peer/common"
)

//This is where all initializations take place. These closely follow CLI
//initializations.

func initCCCheckerParams(configFile string) {
	err := LoadCCCheckerParams(configFile)
	if err != nil {
		fmt.Printf("error unmarshalling ccchecker: %s\n", err)
		os.Exit(1)
	}
}

//initYaml read yaml file env.ConfigDir. Defaults to ../../peer
func initYaml(env *CCCEnv) {
	// For environment variables.
	viper.SetEnvPrefix(cmdRoot)
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AddConfigPath(env.ConfigDir)
	viper.AddConfigPath("./")

	// Now set the configuration file.
	viper.SetConfigName(cmdRoot) // Name of config file (without extension)

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		fmt.Printf("Fatal error when reading %s config file: %s\n", cmdRoot, err)
		os.Exit(2)
	}
}

//initMSP from env.ConfigDir. Defaults to ../../msp/sampleconfig
func initMSP(env *CCCEnv) {
	err := common.InitCrypto(env.MSPDir, env.MSPID)
	if err != nil {
		panic(err.Error())
	}
}

//InitCCCheckerEnv initialize the CCChecker environment
func InitCCCheckerEnv(configFile, envFile string) *CCCEnv {
	initCCCheckerParams(configFile)
	env := loadCCCEnv(envFile)
	initYaml(env)
	initMSP(env)

	return env
}
