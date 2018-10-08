// Copyright © 2018 J. Strobus White.
// This file is part of the blocktop blockchain development kit.
//
// Blocktop is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Blocktop is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with blocktop. If not, see <http://www.gnu.org/licenses/>.

package kernel

import spec "github.com/blocktop/go-spec"

type KernelConfig struct {
	BlockchainName             string
	// BlockRate is expressed in units of blocks per second.
	BlockFrequency             float64
	BlockConfirmer             BlockConfirmer
	CompetitionEvaluator       CompetitionEvaluator
	BlockGenerator             BlockGenerator
	GenesisGenerator           GenesisGenerator
	BlockPrototype             spec.Marshalled
	NetworkNode                spec.NetworkNode
	BlockAdder                 BlockAdder
}

type BlockConfirmer func()
type CompetitionEvaluator func() spec.Competition
type BlockGenerator func(branch []spec.Block, rootID int) spec.Block
type GenesisGenerator func() spec.Block
type BlockAdder func(blocks []spec.Block, local bool) (spec.Block, error)

func (c *KernelConfig) valid() bool {
	return c.BlockConfirmer != nil && c.CompetitionEvaluator != nil &&
		c.BlockGenerator != nil && c.NetworkNode != nil && c.BlockFrequency > 0 &&
		c.BlockchainName != "" && c.BlockPrototype != nil && c.BlockAdder != nil &&
		c.GenesisGenerator != nil
}