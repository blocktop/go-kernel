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

package blockchain

import (
	"context"

	"github.com/golang/glog"

	spec "github.com/blckit/go-spec"
)

type Controller struct {
	Blockchains         map[string]spec.Blockchain
	TransactionHandlers map[string]spec.TransactionHandler
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
	c.TransactionHandlers = make(map[string]spec.TransactionHandler)
	c.startedBlockchains = make(map[string]bool, 0)
	c.stopProc = make([]chan bool, 4)
	for i := 0; i < 4; i++ {
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

func (c *Controller) AddTransactionHandler(h spec.TransactionHandler) {
	txnType := h.GetType()
	if c.TransactionHandlers[txnType] != nil {
		panic("transaction handler already added")
	}
	c.TransactionHandlers[txnType] = h
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

	go c.confirmBlocks(ctx, name)
	go c.confirmLocalBlocks(ctx, name)
	go c.broadcastBlocks(ctx, name)

	c.Blockchains[name].Start(ctx)
}

func (c *Controller) StopBlockchain(name string) {
	c.startedBlockchains[name] = false
	c.Blockchains[name].Stop()

	for i := 0; i < 4; i++ {
		c.stopProc[i] <- true
	}
}

func (c *Controller) confirmBlocks(ctx context.Context, bcType string) {
	ch := c.Blockchains[bcType].GetConfirmChan()
	for {
		select {
		case <-c.stopProc[0]:
		case <-ctx.Done():
			return
		case block := <-ch:
			c.confirmBlock(bcType, block)
		}
	}
}

func (c *Controller) confirmLocalBlocks(ctx context.Context, bcType string) {
	ch := c.Blockchains[bcType].GetConfirmLocalChan()
	for {
		//time.Sleep(100 * time.Millisecond)
		select {
		case <-c.stopProc[1]:
		case <-ctx.Done():
			return
		case block := <-ch:
			c.confirmBlock(bcType, block)
			// hooray!
			// TODO: Log our success and our reward
			glog.Warningf("Peer %s: %s local block %s confirmed, reward = %d", c.logPeerID, bcType, block.GetID()[:6], 1000000000)
		}
	}
}

func (c *Controller) broadcastBlocks(ctx context.Context, bcType string) {
	ch := c.Blockchains[bcType].GetBroadcastChan()
	for {
		//time.Sleep(100 * time.Millisecond)
		select {
		case <-c.stopProc[2]:
		case <-ctx.Done():
			return
		case broadcast := <-ch:
			c.broadcastBlock(ctx, bcType, broadcast)
		}
	}
}

func (c *Controller) confirmBlock(bcType string, block spec.Block) {
	glog.Warningf("Peer %s: %s confirmed block %d: %s", c.logPeerID, bcType, block.GetBlockNumber(), block.GetID()[:6])
	go c.unlogTransactions(bcType, block)
	go c.executeTransactions(bcType, block)
}

func (c *Controller) unlogTransactions(bcType string, block spec.Block) {
	bc := c.Blockchains[bcType]
	if bc == nil {
		// TODO: log something, fail?
		return
	}
	bc.GetBlockGenerator().UnlogTransactions(block.GetTransactions())
}

func (c *Controller) executeTransactions(bcType string, block spec.Block) {
	for _, t := range block.GetTransactions() {
		txnType := t.GetType()
		handler := c.TransactionHandlers[txnType]
		if handler == nil {
			// TODO: log something here
			// if we can't confirm txn then our data will be corrupt
			// or no one else will be able to either
			// or could be security issue
		} else {
			handler.Execute(t)
		}
	}
}

func (c *Controller) broadcastBlock(ctx context.Context, bcType string, broadcast *spec.BroadcastBlock) {
	block := broadcast.Block

	p := &spec.MessageProtocol{}
	p.SetBlockchainType(bcType)
	p.SetResourceType(spec.ResourceTypeBlock)
	p.SetComponentType(block.GetType())
	p.SetVersion(block.GetVersion())

	message := block.Marshal()

	c.NetworkNode.Broadcast(ctx, message, broadcast.From, p)
}

func (c *Controller) receiveMessages(ctx context.Context) {
	ch := c.NetworkNode.GetReceiveChan()
	for {
		//time.Sleep(50 * time.Millisecond)
		select {
		case <-c.stopProc[3]:
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
		go c.receiveTransaction(bc, netMsg)

	case "block":
		go c.receiveBlock(bc, netMsg)
	}
}

func (c *Controller) receiveTransaction(bc spec.Blockchain, netMsg *spec.NetworkMessage) {
	p := netMsg.Protocol
	msg := netMsg.Message

	txnType := p.GetComponentType()
	h := c.TransactionHandlers[txnType]
	if h == nil {
		//TODO
		return
	}
	txn := h.Unmarshal(msg)
	bc.GetBlockGenerator().LogTransaction(txn)
}

func (c *Controller) receiveBlock(bc spec.Blockchain, netMsg *spec.NetworkMessage) {
	msg := netMsg.Message
	block := bc.GetBlockGenerator().Unmarshal(msg, c.TransactionHandlers)
	glog.V(1).Infof("Peer %s: %s received block %s", c.logPeerID, bc.GetType(), block.GetID()[:6])

	broadcast := &spec.BroadcastBlock{
		Block: block,
		From:  netMsg.From}

	bc.GetReceiveChan() <- broadcast
}
