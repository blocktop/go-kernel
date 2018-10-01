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

	push "github.com/blocktop/go-push-components"
	spec "github.com/blocktop/go-spec"
	"github.com/fatih/color"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

type Controller struct {
	Blockchains        map[string]spec.Blockchain
	NetworkNode        spec.NetworkNode
	PeerID             string
	logPeerID          string
	startedBlockchains map[string]bool
	started            bool
	receiveQ           *push.PushQueue
}

// compile-time check that interface is satisfied
var _ spec.Controller = (*Controller)(nil)

func NewController(networkNode spec.NetworkNode) *Controller {
	c := &Controller{NetworkNode: networkNode, PeerID: networkNode.PeerID()}
	c.logPeerID = c.PeerID[2:8] // remove the "Qm" and take 6 runes
	c.Blockchains = make(map[string]spec.Blockchain)
	c.startedBlockchains = make(map[string]bool, 0)

	c.setupMessageReceiver()

	return c
}

func (c *Controller) setupMessageReceiver() {
	c.NetworkNode.OnMessageReceived(c.networkMessageHandler)

	concurrency := viper.GetInt("blockchain.receiveconcurrency")
	c.receiveQ = push.NewPushQueue(concurrency, 1000, func(msg push.QueueItem) {
		netMsg, ok := msg.(*spec.NetworkMessage)
		if !ok {
			glog.Warningf("Peer %s: received wrong message type from queue", c.logPeerID)
			return
		}
		c.receiveMessage(netMsg)
	})

	c.receiveQ.OnOverload(func(item push.QueueItem) {
		glog.Errorln(color.HiRedString("Peer %s: network receive queue was overloaded", c.logPeerID))
	})

	c.receiveQ.Start()
}

func (c *Controller) AddBlockchain(bc spec.Blockchain) {
	bcType := bc.Type()
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
}

func (c *Controller) Stop() {
	c.started = false

	for name := range c.Blockchains {
		c.StopBlockchain(name)
	}
}

func (c *Controller) StartBlockchain(ctx context.Context, name string) {
	c.startedBlockchains[name] = true

	c.Blockchains[name].Start(ctx, c.NetworkNode.Broadcast)
}

func (c *Controller) StopBlockchain(name string) {
	c.startedBlockchains[name] = false
	c.Blockchains[name].Stop()
}

func (c *Controller) networkMessageHandler(netMsg *spec.NetworkMessage) {
	c.receiveQ.Put(netMsg)
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
		bc.ReceiveTransaction(netMsg)

	case "block":
		bc.ReceiveBlock(netMsg)
	}
}
