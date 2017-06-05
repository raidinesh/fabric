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
	"sync"
	"time"

	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/peer/chaincode"
	"github.com/hyperledger/fabric/peer/common"
	pb "github.com/hyperledger/fabric/protos/peer"

	"golang.org/x/net/context"
)

//ShadowCCIntf interfaces to be implemented by shadow chaincodes
type ShadowCCIntf interface {
	//----- initialization and chaincode invocation phase -----
	//Clone clones
	Clone() interface{}

	//InitShadowCC initializes a shadow
	InitShadowCC(ccname string, initArgs []string)

	//GetInvokeArgs gets invoke arguments from shadow
	GetInvokeArgs(ccnum int, iter int) [][]byte

	//PostInvoke passes the retvalue from the invoke to the shadow for post-processing
	PostInvoke(args [][]byte, retval []byte) error

	//OverrideNumInvokes overrides the users number of invoke request if the "NumberOfInvokes"
	//JSON property is incompatible or incorrect for this shadow for some reason
	OverrideNumInvokes(numInvokesPlanned int) int

	//----- validation phase -----
	//InitValidation initializes the shadow for validation
	InitValidation() error

	//NextQueryArgs returns the next args  to call query with
	//Note - queries can be concurrent, so this func should be reentrant
	NextQueryArgs() [][]byte

	//Validate the results against the query arguments
	Validate(args [][]byte, result []byte) error

	//ValidationDone cleans up and does any final tallying
	ValidationDone() error
}

//CCClient chaincode properties, config and runtime
type CCClient struct {
	sync.Mutex
	//-------------config properties ------------
	//Name of the chaincode
	Name string

	//InitArgs used for deploying the chaincode
	InitArgs []string

	//Path to the chaincode
	Path string

	//NumFinalQueryAttempts number of times to try final query before giving up
	NumFinalQueryAttempts int

	//NumberOfInvokes number of iterations to do invoke on
	NumberOfInvokes int

	//DelayBetweenInvokeMs delay between each invoke
	DelayBetweenInvokeMs int

	//DelayBetweenQueryMs delay between each query
	DelayBetweenQueryMs int

	//TimeoutToAbortRunSecs timeout for aborting this chaincode processing during run
	TimeoutToAbortRunSecs int

	//TimeoutToAbortVerifySecs timeout for aborting this chaincode processing during verify
	TimeoutToAbortVerifySecs int

	//Lang of chaincode
	Lang string

	//WaitAfterInvokeMs wait time before validating invokes for this chaincode
	WaitAfterInvokeMs int

	//Concurrency number of goroutines to spin
	Concurrency int

	//-------------runtime properties ------------
	//Unique number assigned to this CC by CCChecker
	ID int

	//shadow CC where the chaincode stats is maintained
	shadowCC ShadowCCIntf

	//number of iterations a shadow cc actually wants
	//this could be different from NumberOfInvokes
	overriddenNumInvokes int

	//current iteration of invoke
	currentInvokeIter int

	//start of invokes in epoch seconds
	invokeStartTime int64

	//end of invokes in epoch seconds
	invokeEndTime int64

	//total invokes
	numInvokes int

	//num successful invokes
	numSuccessfulInvokes int

	//error on a invoke in an iteration
	invokeErrs []error

	//total queries
	numQueries int

	//num successful queries
	numSuccessfulQueries int

	//error on a query in an iteration
	queryErrs []error
}

func (cc *CCClient) getChaincodeSpec(args [][]byte) *pb.ChaincodeSpec {
	return &pb.ChaincodeSpec{
		Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[cc.Lang]),
		ChaincodeId: &pb.ChaincodeID{Path: cc.Path, Name: cc.Name},
		Input:       &pb.ChaincodeInput{Args: args},
	}
}

//doInvokes calls invoke for each iteration for the chaincode
//Stops at the first invoke with error
//currentInvokeIter contains the number of successful iterations
func (cc *CCClient) doInvokes(ctxt context.Context, id int, iter int, chainID string,
	bc common.BroadcastClient, ec pb.EndorserClient, signer msp.SigningIdentity,
	wg *sync.WaitGroup, quit func() bool) (int, error) {

	var err error
	var i int
	for i = 0; i < iter; i++ {
		if quit() {
			break
		}

		//concurrent id of this chaincode to be used as a unique id
		//along with this iteration
		args := cc.shadowCC.GetInvokeArgs(id, i)

		spec := cc.getChaincodeSpec(args)

		if quit() {
			break
		}

		var pResp *pb.ProposalResponse
		if pResp, err = chaincode.ChaincodeInvokeOrQuery(spec, chainID, true, signer, ec, bc); err != nil {
			break
		}

		resp := pResp.Response.Payload
		if err = cc.shadowCC.PostInvoke(args, resp); err != nil {
			break
		}

		if quit() {
			break
		}

		//don't sleep for the last iter
		if cc.DelayBetweenInvokeMs > 0 && cc.currentInvokeIter < (cc.overriddenNumInvokes-1) {
			time.Sleep(time.Duration(cc.DelayBetweenInvokeMs) * time.Millisecond)
		}
	}

	return i, err
}

func (cc *CCClient) invokeDone(succ int, total int, err error) {
	cc.Lock()
	defer cc.Unlock()

	cc.numInvokes = cc.numInvokes + total
	cc.numSuccessfulInvokes = cc.numSuccessfulInvokes + succ
	if err != nil {
		cc.invokeErrs = append(cc.invokeErrs, err)
	}
}

//Run test over given number of iterations
//  i will be unique across chaincodes and can be used as a key
//    this is useful if chaincode occurs multiple times in the array of chaincodes
func (cc *CCClient) Run(ctxt context.Context, chainID string, bc common.BroadcastClient, ec pb.EndorserClient, signer msp.SigningIdentity, wg *sync.WaitGroup) error {
	var (
		quit bool
		err  error
	)

	//capture the error
	defer func() {
		wg.Done()
		//indication to ignore the run (elapsed time will be -ve)
		if err != nil {
			cc.invokeEndTime = 0
		}
	}()

	var innerwg sync.WaitGroup
	innerwg.Add(cc.Concurrency)

	//return the quit closure for validation within validateIter
	quitF := func() bool { return quit }

	//perhaps the shadow CC wants to override the number of iterations
	cc.overriddenNumInvokes = cc.shadowCC.OverrideNumInvokes(cc.NumberOfInvokes)

	//start of invokes
	cc.invokeStartTime = time.Now().UnixNano() / 1000000

	//invoke concurrently, bail on the first error
	for i := 0; i < cc.Concurrency; i++ {
		go func(id int) {
			defer innerwg.Done()
			succ, invokeErr := cc.doInvokes(ctxt, id, cc.overriddenNumInvokes, chainID, bc, ec, signer, wg, quitF)
			cc.invokeDone(succ, cc.overriddenNumInvokes, invokeErr)
			if invokeErr != nil {
				//signal other routines too
				quit = true
			}
		}(i)
	}

	//shouldn't block the sender go routine on cleanup
	iDone := make(chan struct{}, 1)

	//wait for the above invokes to be done
	go func() {
		innerwg.Wait()
		cc.invokeEndTime = time.Now().UnixNano() / 1000000
		iDone <- struct{}{}
	}()

	//end of invokes

	//we could be done or cancelled or timedout
	select {
	case <-ctxt.Done():
		quit = true
	case <-iDone:
	case <-time.After(time.Duration(cc.TimeoutToAbortRunSecs) * time.Second):
		quit = true
		err = fmt.Errorf("Aborting due to timeoutout!!")
	}

	//note down the errors
	if len(cc.invokeErrs) > 0 {
		if err == nil {
			err = fmt.Errorf("Found %d invoke errors", len(cc.invokeErrs))
		} else {
			err = fmt.Errorf("Found %d invoke errors and a non-invoke error %s", len(cc.invokeErrs), err)
		}
	}

	return err
}

//doQuery a query on the chaincode, the args are supplied by the shadow
func (cc *CCClient) doQuery(ctxt context.Context, args [][]byte, chainID string, bc common.BroadcastClient, ec pb.EndorserClient, signer msp.SigningIdentity, quit func() bool) error {
	spec := cc.getChaincodeSpec(args)

	var err error

	//lets try a few times
	for i := 0; i < cc.NumFinalQueryAttempts; i++ {
		if quit() {
			break
		}

		var pResp *pb.ProposalResponse
		if pResp, err = chaincode.ChaincodeInvokeOrQuery(spec, chainID, false, signer, ec, bc); err != nil {
			break
		}

		if quit() {
			break
		}

		//if it fails, we try again
		if err = cc.shadowCC.Validate(args, pResp.Response.Payload); err == nil {
			break
		}

		if quit() {
			break
		}

		//try again
		if cc.DelayBetweenQueryMs > 0 {
			time.Sleep(time.Duration(cc.DelayBetweenQueryMs) * time.Millisecond)
		}
	}

	return err
}

func (cc *CCClient) queryDone(err error) {
	cc.Lock()
	defer cc.Unlock()

	cc.numQueries = cc.numQueries + 1
	if err != nil {
		cc.queryErrs = append(cc.queryErrs, err)
	} else {
		cc.numSuccessfulQueries = cc.numSuccessfulQueries + 1
	}
}

//Validate test that was Run. Each successful iteration in the run is validated against
func (cc *CCClient) Validate(ctxt context.Context, chainID string, bc common.BroadcastClient, ec pb.EndorserClient, signer msp.SigningIdentity, wg *sync.WaitGroup) error {
	defer wg.Done()

	//this will signal inner validators to get out via
	//closure
	var quit bool

	//initialize the shadow for validation.. could fail, if so bail
	//err will be returned to caller
	if err := cc.shadowCC.InitValidation(); err != nil {
		return err
	}

	var innerwg sync.WaitGroup
	innerwg.Add(cc.Concurrency)

	//initialize for querying
	cc.queryErrs = []error{}

	//give some time for the invokes to commit for this cc
	time.Sleep(time.Duration(cc.WaitAfterInvokeMs) * time.Millisecond)

	//return the quit closure for validation within validateIter
	quitF := func() bool { return quit }

	//error to return
	var err error

	//invoke queries concurrently on the shadow
	for i := 0; i < cc.Concurrency; i++ {
		go func() {
			defer innerwg.Done()
			for {
				args := cc.shadowCC.NextQueryArgs()
				if args == nil {
					break
				}
				qErr := cc.doQuery(ctxt, args, chainID, bc, ec, signer, quitF)
				cc.queryDone(qErr)
			}
		}()
	}

	//shouldn't block the sender go routine on cleanup
	qDone := make(chan struct{}, 1)

	//wait for the above queries to be done
	go func() { innerwg.Wait(); qDone <- struct{}{} }()

	//we could be done or cancelled or timedout
	select {
	case <-ctxt.Done():
		//we don't know why it was cancelled but it was cancelled
		quit = true
	case <-qDone:
		//final validation and cleanup
		err = cc.shadowCC.ValidationDone()
	case <-time.After(time.Duration(cc.TimeoutToAbortVerifySecs) * time.Second):
		quit = true
		err = fmt.Errorf("Aborting due to timeoutout!!")
	}

	return err
}

//Report reports chaincode test execution, iter by iter
func (cc *CCClient) Report(verbose bool, chainID string) {
	//total actual time for cc.currentInvokeIter
	invokeTime := cc.invokeEndTime - cc.invokeStartTime - int64(cc.DelayBetweenInvokeMs*(cc.currentInvokeIter-1))
	fmt.Printf("\tTime for invokes(ms): %d\n", invokeTime)

	for _, err := range cc.invokeErrs {
		fmt.Printf("\tInvoke error : %s\n", err)
	}
	fmt.Printf("\tNum successful invokes: %d(%d)\n", cc.numSuccessfulInvokes, cc.numInvokes)

	for _, err := range cc.queryErrs {
		fmt.Printf("\tQuery error : %s\n", err)
	}
	fmt.Printf("\tNum successful queries: %d(%d)\n", cc.numSuccessfulQueries, cc.numQueries)
}
