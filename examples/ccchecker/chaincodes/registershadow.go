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

package chaincodes

import (
	"fmt"

	//shadow chaincodes to be registered
	nkpi "github.com/hyperledger/fabric/examples/ccchecker/chaincodes/newkeyperinvoke/shadow"
)

//all the statically registered shadow chaincodes that can be used
var shadowCCs = map[string]ShadowCCIntf{
	"github.com/hyperledger/fabric/examples/ccchecker/chaincodes/newkeyperinvoke": &nkpi.NewKeyPerInvoke{},
}

// For each chaincode that is to be tested corresponds a shadowCC. That shadow
// is imported in the list of imports above. Each shadow is linked to an actual
// chaincode via the later's path. The path is mapped to the shadow in shadowCCs
// above. The path is also a property in the CCClient. Thus the shadow is linked
// with the CCClient in RegisterCCClients.
//
// Note 1 - in the model, CCChecker is not involved in bringing up the peers or
// initializing chaincodes in them. CCChecker assumes chaincodes are launched and
// ready for test. CCChecker is used only to do Invokes and involve shadows to
// keep track of the expected results.
//
// Note 2 - not all paths in shadowCCs maybe in the CCClient array (ie, in the
// input JSON file). In other words while the shadowCCs contain a list of all
// possible Chaincodes that may be tested, only a subset of that may actually
// be.
//
// Note 3 - each shadowCC maybe used multiple times (either because Concurrency
// is > 1 or the chaincode path is used multiple times in the JSON with different
// CCChecker parameters).

// inUseShadowCCs keeps a set of all shadow CCs in use (see Note 2 above)
var inUseShadowCCs map[string]ShadowCCIntf

// RegisterCCClients registers and maps chaincode clients to their shadows
func RegisterCCClients(ccs []*CCClient) error {
	inUseShadowCCs = make(map[string]ShadowCCIntf)
	for _, cc := range ccs {
		shadowTemplate, ok := shadowCCs[cc.Path]
		if !ok || shadowTemplate == nil {
			return fmt.Errorf("%s not a registered chaincode", cc.Path)
		}

		if _, ok = inUseShadowCCs[cc.Name]; ok {
			return fmt.Errorf("Duplicate chaincode entry %s", cc.Name)
		}

		// setup the shadow chaincode to plug into the ccchecker framework
		cc.shadowCC = shadowTemplate.Clone().(ShadowCCIntf)

		// initialize the shadow ... not to be confused with initializing the
		// actual chaincode
		cc.shadowCC.InitShadowCC(cc.Name, cc.InitArgs)

		inUseShadowCCs[cc.Name] = cc.shadowCC
	}

	return nil
}

//ListShadowCCs lists all registered shadow ccs in the library
func ListShadowCCs() {
	for key := range shadowCCs {
		fmt.Printf("\t%s\n", key)
	}
}

//GetInUseShadowCCs returns the in use shadows (mainly used for validating)
func GetInUseShadowCCs() map[string]ShadowCCIntf {
	return inUseShadowCCs
}
