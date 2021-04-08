/*
 * Flow CLI
 *
 * Copyright 2019-2021 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package services

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-cli/pkg/flowcli/contracts"
	"github.com/onflow/flow-cli/pkg/flowcli/gateway"
	"github.com/onflow/flow-cli/pkg/flowcli/output"
	"github.com/onflow/flow-cli/pkg/flowcli/project"
	"github.com/onflow/flow-cli/pkg/flowcli/util"
)

// Project is a service that handles all interactions for a project.
type Project struct {
	gateway gateway.Gateway
	project *project.Project
	logger  output.Logger
}

// NewProject returns a new project service.
func NewProject(
	gateway gateway.Gateway,
	project *project.Project,
	logger output.Logger,
) *Project {
	return &Project{
		gateway: gateway,
		project: project,
		logger:  logger,
	}
}

func (p *Project) Init(
	reset bool,
	serviceKeySigAlgo string,
	serviceKeyHashAlgo string,
	servicePrivateKey string,
) (*project.Project, error) {
	if !project.Exists(project.DefaultConfigPath) || reset {

		sigAlgo, hashAlgo, err := util.ConvertSigAndHashAlgo(serviceKeySigAlgo, serviceKeyHashAlgo)
		if err != nil {
			return nil, err
		}

		proj, err := project.Init(sigAlgo, hashAlgo)
		if err != nil {
			return nil, err
		}

		if len(servicePrivateKey) > 0 {
			serviceKey, err := crypto.DecodePrivateKeyHex(sigAlgo, servicePrivateKey)
			if err != nil {
				return nil, fmt.Errorf("could not decode private key for a service account, provided private key: %s", servicePrivateKey)
			}

			proj.SetEmulatorServiceKey(serviceKey)
		}

		err = proj.Save(project.DefaultConfigPath)
		if err != nil {
			return nil, err
		}

		return proj, nil
	}

	return nil, fmt.Errorf(
		"configuration already exists at: %s, if you want to reset configuration use the reset flag",
		project.DefaultConfigPath,
	)
}

func (p *Project) Deploy(network string, update bool) ([]*contracts.Contract, error) {
	if p.project == nil {
		return nil, fmt.Errorf("missing configuration, initialize it: flow init")
	}

	// check there are not multiple accounts with same contract
	// TODO: specify which contract by name is a problem
	if p.project.ContractConflictExists(network) {
		return nil, fmt.Errorf(
			"the same contract cannot be deployed to multiple accounts on the same network",
		)
	}

	processor := contracts.NewPreprocessor(
		contracts.FilesystemLoader{},
		p.project.AliasesForNetwork(network),
	)

	for _, contract := range p.project.ContractsByNetwork(network) {
		err := processor.AddContractSource(
			contract.Name,
			contract.Source,
			contract.Target,
		)
		if err != nil {
			return nil, err
		}
	}

	err := processor.ResolveImports()
	if err != nil {
		return nil, err
	}

	orderedContracts, err := processor.ContractDeploymentOrder()
	if err != nil {
		return nil, err
	}

	p.logger.Info(fmt.Sprintf(
		"\nDeploying %d contracts for accounts: %s\n",
		len(orderedContracts),
		strings.Join(p.project.AccountNamesForNetwork(network), ","),
	))

	deployErr := false
	for _, contract := range orderedContracts {
		targetAccount := p.project.AccountByAddress(contract.Target().String())

		if targetAccount == nil {
			return nil, fmt.Errorf("target account for deploying contract not found in configuration")
		}

		targetAccountInfo, err := p.gateway.GetAccount(targetAccount.Address())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch information for account %s with error %s", targetAccount.Address(), err.Error())
		}

		var tx *flow.Transaction

		_, exists := targetAccountInfo.Contracts[contract.Name()]
		if exists {
			if !update {
				p.logger.Error(fmt.Sprintf(
					"contract %s is already deployed to this account. Use the --update flag to force update",
					contract.Name(),
				))

				deployErr = true
				continue
			}

			tx = prepareUpdateContractTransaction(targetAccount.Address(), contract)
		} else {
			tx = prepareAddContractTransaction(targetAccount.Address(), contract)
		}

		tx, err = p.gateway.SendTransaction(tx, targetAccount)
		if err != nil {
			p.logger.Error(err.Error())
			deployErr = true
		}

		p.logger.StartProgress(
			fmt.Sprintf("%s deploying...", util.Bold(contract.Name())),
		)

		result, err := p.gateway.GetTransactionResult(tx, true)
		if err != nil {
			p.logger.Error(err.Error())
			deployErr = true
		}

		if result.Error == nil && !deployErr {
			p.logger.StopProgress(
				fmt.Sprintf("%s -> 0x%s", util.Green(contract.Name()), contract.Target()),
			)
		} else {
			p.logger.StopProgress(
				fmt.Sprintf("%s error", util.Red(contract.Name())),
			)
			p.logger.Error(
				fmt.Sprintf("%s error: %s", contract.Name(), result.Error),
			)
		}
	}

	if !deployErr {
		p.logger.Info("\n✨  All contracts deployed successfully")
	} else {
		err = fmt.Errorf("failed to deploy contracts")
		p.logger.Error(err.Error())
		return nil, err
	}

	return orderedContracts, nil
}

func prepareUpdateContractTransaction(
	targetAccount flow.Address,
	contract *contracts.Contract,
) *flow.Transaction {
	return templates.UpdateAccountContract(
		targetAccount,
		templates.Contract{
			Name:   contract.Name(),
			Source: contract.TranspiledCode(),
		},
	)
}

func prepareAddContractTransaction(
	targetAccount flow.Address,
	contract *contracts.Contract,
) *flow.Transaction {
	return templates.AddAccountContract(
		targetAccount,
		templates.Contract{
			Name:   contract.Name(),
			Source: contract.TranspiledCode(),
		},
	)
}