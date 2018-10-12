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

import (
	"time"

	"github.com/spf13/viper"

	spec "github.com/blocktop/go-spec"
	"github.com/golang/glog"
)

type KernelBlock struct {
	proto   spec.Marshalled
	blockQs *blockQueues
	msgChan *MessageChannel
	comp    spec.Competition
	gen     BlockGenerator
	genesis GenesisGenerator
	eval    CompetitionEvaluator
	conf    BlockConfirmer
	add     BlockAdder
	genNum  uint64
}

var blk *KernelBlock

func initBlock(c *KernelConfig) {
	b := &KernelBlock{}
	b.proto = c.BlockPrototype
	b.gen = c.BlockGenerator
	b.eval = c.CompetitionEvaluator
	b.conf = c.BlockConfirmer
	b.add = c.BlockAdder
	b.msgChan = NewMessageChannel(b.proto, b.recvHandler)
	b.blockQs = newBlockQueues()

	if viper.GetBool("blockchain.genesis") {
		b.genesis = c.GenesisGenerator
	}

	net.RegisterMessageChannel(b.msgChan)

	blk = b
}

func (b *KernelBlock) BlockNumber() uint64 {
	return b.genNum
}

func (b *KernelBlock) start() {
	glog.V(3).Infof("%s: resuming new block processing", ktime.String())
	b.blockQs.start()
}

func (b *KernelBlock) stop() {
	glog.V(3).Infof("%s: suspending new block processing", ktime.String())
	b.blockQs.stop()
}

func (b *KernelBlock) maint() {
	confStartTime := time.Now().UnixNano()
	glog.V(3).Infof("%s: running block confirmer", ktime.String())
	b.conf()
	confEndTime := time.Now().UnixNano()
	metrics.setConfBlockTime(confEndTime - confStartTime)

	evalStartTime := time.Now().UnixNano()
	glog.V(3).Infof("%s: running head block evaluator", ktime.String())
	comp := b.eval()
	evalEndTime := time.Now().UnixNano()
	metrics.setEvalTime(evalEndTime - evalStartTime)

	b.setComp(comp)

	metrics.setBlockQCount(b.blockQs.count())
}

func (b *KernelBlock) setComp(comp spec.Competition) {
	b.comp = comp
}

func (b *KernelBlock) generate() {
	glog.V(3).Infof("%s: initiating block generation", ktime.String())
	if b.genesis != nil {
		newBlock := b.genesis()
		b.genesis = nil
		b.outputNewLocalBlock(newBlock)
		return
	}

	nocomp := true
	if b.comp != nil {
		// Get the branch we are to generate on.
		branch, rootID, switchHeads := b.comp.Branch(b.genNum)
		// TODO: if switchHeads find latest block generated on the new head, if any.
		// Generate on that.
		_ = switchHeads
		var newBlock spec.Block
		if branch != nil {
			b.genNum = branch[0].BlockNumber() + 1
			startTime := time.Now().UnixNano()
			newBlock = b.gen(branch, rootID)
			endTime := time.Now().UnixNano()
			metrics.setGenBlockTime(endTime - startTime)
			if !b.outputNewLocalBlock(newBlock) {
				return
			}
			nocomp = false
		}
	}
	if nocomp {
		glog.V(3).Infof("%s: no competition at block %d", ktime.String(), b.genNum)
	}
}

func (b *KernelBlock) outputNewLocalBlock(newBlock spec.Block) bool {
	netMsg, err := b.makeNetMsg(newBlock)
	if err != nil {
		glog.Error("Failed to make net message from newly generated block")
		return false
	}
	glog.V(3).Infof("%s: generated local block %d:%s", ktime.String(), newBlock.BlockNumber(), newBlock.Hash()[:6])

	// Locally-generated block bypasses the queues.
	addedBlock, err := b.add([]spec.Block{newBlock}, true)
	if err != nil {
		glog.Errorln("Failed to add locally-generated block to consensus")
	}
	if addedBlock != nil {
		net.priorityBroadcast(netMsg)
	}
	return true
}

func (b *KernelBlock) recvHandler(netMsg *spec.NetworkMessage) {
	panicIfUninitialized()
	block, err := b.msgChan.unmarshal(netMsg)
	if err != nil {
		glog.Errorf("Failed to unmarshal block message from %s", netMsg.From[:6])
		return
	}

	if block.Hash() != netMsg.Hash {
		glog.Errorln("block data does not match message hash from", netMsg.From[:6])
		return
	}

	b.blockQs.put(block.(spec.Block), netMsg)
}

func (k *Kernel) transactionMessageReceiver(netMsg *spec.NetworkMessage) {

}

func (b *KernelBlock) blockBatchWorker(items []*blockQueueItem, local bool) {
	blocks := make([]spec.Block, len(items))
	index := make(map[string]*spec.NetworkMessage)

	for i, item := range items {
		blocks[i] = item.block
		index[item.block.Hash()] = item.netMsg
	}
	startTime := time.Now().UnixNano()
	addedBlock, err := b.add(blocks, local)
	if err != nil {
		glog.Errorln("failed to add blocks:", err)
		return
	}
	endTime := time.Now().UnixNano()
	metrics.setAddBlockTime(endTime - startTime)

	if addedBlock != nil {
		netMsg := index[addedBlock.Hash()]
		net.priorityBroadcast(netMsg)
	}
}

func (b *KernelBlock) makeNetMsg(block spec.Block) (*spec.NetworkMessage, error) {
	data, links, err := block.Marshal()
	if err != nil {
		return nil, err
	}
	netMsg := &spec.NetworkMessage{
		Data:     data,
		Links:    links,
		Hash:     block.Hash(),
		Protocol: b.msgChan.Protocol,
		From:     net.PeerID()}

	//fmt.Printf("%v\n", netMsg)
	return netMsg, nil
}
