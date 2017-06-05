/*
Copyright IBM Corp. 2017 All Rights Reserved.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
)

//CCCEnv encapsulates environment for ccchecker
type CCCEnv struct {
	//Orderer ip:port
	Orderer   string

	//ConfigDir path to peer yaml file
	ConfigDir string

	//MSPDir directory to the local MSP
	MSPDir    string

	//MSPID msp ID to use
	MSPID     string
}

//loadCCCEnv reads in the Env given env file (default env.json)
func loadCCCEnv(file string) *CCCEnv {
	var b []byte
	var err error

	if b, err = ioutil.ReadFile(file); err != nil {
		panic(fmt.Sprintf("Cannot read env file %s\n", err))

	}

	env := &CCCEnv{}
	err = json.Unmarshal(b, &env)
	if err != nil {
		panic(fmt.Sprintf("error unmarshalling cccenv: %s\n", err))
	}

	return env
}
