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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/examples/ccchecker/chaincodes"
	"github.com/hyperledger/fabric/peer/common"
)

//global ccchecker params
var ccchecker *CCChecker

//CCChecker encapsulates ccchecker properties and runtime
type CCChecker struct {
	//ChaincodeClients to be used in ccchecker (see ccchecker.json for defaults)
	ChaincodeClients []*chaincodes.CCClient
	//TimeoutToAbortRunSecs abort deadline for run
	TimeoutToAbortRunSecs int
	//TimeoutToAbortVerifySecs abort deadline for verify
	TimeoutToAbortVerifySecs int
	//ChainName name of the chain
	ChainName string
}

//LoadCCCheckerParams read the ccchecker params from a file
func LoadCCCheckerParams(file string) error {
	var b []byte
	var err error

	if b, err = ioutil.ReadFile(file); err != nil {
		return fmt.Errorf("Cannot read config file %s\n", err)
	}

	ccc := &CCChecker{}
	err = json.Unmarshal(b, &ccc)
	if err != nil {
		return fmt.Errorf("error unmarshalling ccchecker: %s\n", err)
	}

	//provide an id for each Chaincode to be tested
	id := 0
	for _, scc := range ccc.ChaincodeClients {
		scc.ID = id
		id = id + 1
	}

	ccchecker = ccc

	return nil
}

//CCCheckerInit assigns shadow chaincode to each of the CCClient from registered shadow chaincodes
func CCCheckerInit() {
	if ccchecker == nil {
		fmt.Printf("LoadCCCheckerParams needs to be called before init\n")
		os.Exit(1)
	}

	if err := chaincodes.RegisterCCClients(ccchecker.ChaincodeClients); err != nil {
		panic(fmt.Sprintf("%s", err))
	}
}

//CCCheckerRun will run the tests and cleanup
//   initialize the env by connecting with orderer and endorser
//   run concurrent invokes obtained from the shadow
//   verify the results as directed by the shadow
//        query the chaincode using queries obtained from the shadow
//        feedback the results of the queries to the shadow
//        shadow to Validate by comparing expected results from the invoke phase with the response from the query
//   report
func CCCheckerRun(env *CCCEnv, report bool, verbose bool) error {
	//connect with Broadcast client
	bc, err := common.GetBroadcastClient(env.Orderer, false, "")
	if err != nil {
		return err
	}
	defer bc.Close()

	ec, err := common.GetEndorserClient()
	if err != nil {
		return err
	}

	signer, err := common.GetDefaultSigner()
	if err != nil {
		return err
	}

	//when the wait's timeout and get out of ccchecker, we
	//cancel and release all goroutines
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ccsWG sync.WaitGroup
	ccsWG.Add(len(ccchecker.ChaincodeClients))

	//an anonymous struct to hold failures
	var failures struct {
		sync.Mutex
		failedCCClients int
	}

	//run the invokes
	ccerrs := make([]error, len(ccchecker.ChaincodeClients))
	for _, cc := range ccchecker.ChaincodeClients {
		go func(cc2 *chaincodes.CCClient) {
			if ccerrs[cc2.ID] = cc2.Run(ctxt, ccchecker.ChainName, bc, ec, signer, &ccsWG); ccerrs[cc2.ID] != nil {
				failures.Lock()
				failures.failedCCClients = failures.failedCCClients + 1
				failures.Unlock()
			}
		}(cc)
	}

	//wait or timeout
	err = ccchecker.wait(&ccsWG, ccchecker.TimeoutToAbortRunSecs)

	//verify results
	if err == nil && failures.failedCCClients < len(ccchecker.ChaincodeClients) {
		ccsWG = sync.WaitGroup{}
		ccsWG.Add(len(ccchecker.ChaincodeClients) - failures.failedCCClients)
		for _, cc := range ccchecker.ChaincodeClients {
			go func(cc2 *chaincodes.CCClient) {
				if ccerrs[cc2.ID] == nil {
					ccerrs[cc2.ID] = cc2.Validate(ctxt, ccchecker.ChainName, bc, ec, signer, &ccsWG)
				} else {
					fmt.Printf("Ignoring [%v] for validation as it returned err %s\n", cc2, ccerrs[cc2.ID])
				}
			}(cc)
		}

		//wait or timeout
		err = ccchecker.wait(&ccsWG, ccchecker.TimeoutToAbortVerifySecs)
	}

	if report {
		for _, cc := range ccchecker.ChaincodeClients {
			cc.Report(verbose, ccchecker.ChainName)
		}
	}

	return err
}

func (s *CCChecker) wait(ccsWG *sync.WaitGroup, ts int) error {
	done := make(chan struct{})
	go func() {
		ccsWG.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(time.Duration(ts) * time.Second):
		return fmt.Errorf("Aborting due to timeoutout!!")
	}
}
