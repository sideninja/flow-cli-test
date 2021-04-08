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

package project

import (
	"github.com/spf13/cobra"

	"github.com/onflow/flow-cli/internal/command"
	"github.com/onflow/flow-cli/pkg/flowcli/contracts"
	"github.com/onflow/flow-cli/pkg/flowcli/services"
)

type flagsDeploy struct {
	Update bool `flag:"update" default:"false" info:"use update flag to update existing contracts"`
}

var deployFlags = flagsDeploy{}

var DeployCommand = &command.Command{
	Cmd: &cobra.Command{
		Use:   "deploy",
		Short: "Deploy Cadence contracts",
	},
	Flags: &deployFlags,
	Run: func(
		cmd *cobra.Command,
		args []string,
		globalFlags command.GlobalFlags,
		services *services.Services,
	) (command.Result, error) {
		c, err := services.Project.Deploy(globalFlags.Network, deployFlags.Update)
		if err != nil {
			return nil, err
		}

		return &DeployResult{c}, nil
	},
}

// DeployResult result structure
type DeployResult struct {
	contracts []*contracts.Contract
}

// JSON convert result to JSON
func (r *DeployResult) JSON() interface{} {
	result := make(map[string]string)

	for _, contract := range r.contracts {
		result[contract.Name()] = contract.Target().String()
	}

	return result
}

// String convert result to string
func (r *DeployResult) String() string {
	return ""
}

// Oneliner show result as one liner grep friendly
func (r *DeployResult) Oneliner() string {
	return ""
}