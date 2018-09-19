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

package controller

import (
	"context"

	spec "github.com/blocktop/go-spec"
)

type Controller struct {
	Blockchains         map[string]spec.Blockchain
	NetworkNode         spec.NetworkNode
	PeerID              string
	logPeerID           string
	startedBlockchains  map[string]bool
	started             bool
	stopProc            []chan bool
}

func NewController(networkNode spec.NetworkNode) *Controller {
	c := &Controller{NetworkNode: networkNode, PeerID: networkNode.GetPeerID()}
	c.logPeerID = c.PeerID[2:8] // remove the "Qm" and take 6 runes
	c.Blockchains = make(map[string]spec.Blockchain)
	c.startedBlockchains = make(map[string]bool, 0)
	c.stopProc = make([]chan bool, 3)
	for i := 0; i < 3; i++ {
		c.stopProc[i] = make(chan bool, 1)
	}

	return c
}

func (c *Controller) AddBlockchain(bc spec.Blockchain) {
	bcType := bc.GetType()
	if c.Blockchains[bcType] != nil {
		panic("blockchain already added")
	}
	c.Blockchains[bcType] = bc
	c.startedBlockchains[bcType] = false
}

func (c *Controller) Start(ctx context.Context) {
	c.started = true

	for name := range c.Blockchains {
		c.StartBlockchain(ctx, name)
	}
	go c.receiveMessages(ctx)
}

func (c *Controller) Stop() {
	c.started = false

	for name := range c.Blockchains {
		c.StopBlockchain(name)
	}
}

func (c *Controller) StartBlockchain(ctx context.Context, name string) {
	c.startedBlockchains[name] = true

	c.Blockchains[name].Start(ctx, c.NetworkNode.GetBroadcastChan())
}

func (c *Controller) StopBlockchain(name string) {
	c.startedBlockchains[name] = false
	c.Blockchains[name].Stop()

	for i := 0; i < 3; i++ {
		c.stopProc[i] <- true
	}
}

func (c *Controller) receiveMessages(ctx context.Context) {
	ch := c.NetworkNode.GetReceiveChan()
	for {
		//time.Sleep(50 * time.Millisecond)
		select {
		case <-c.stopProc[1]:
		case <-ctx.Done():
			return
		case netMsg := <-ch:
			c.receiveMessage(netMsg)
		}
	}
}

func (c *Controller) receiveMessage(netMsg *spec.NetworkMessage) {
	p := netMsg.Protocol
	bcType := p.GetBlockchainType()

	if !p.IsValid() {
		// something not right, what to do?
		return
	}
	bc := c.Blockchains[bcType]
	if bc == nil {
		// TODO: log, fail, what? why is someone sending us this?
		return
	}

	switch p.GetResourceType() {
	case "transaction":
		go bc.ReceiveTransaction(netMsg)

	case "block":
		go bc.ReceiveBlock(netMsg)
	}
}
