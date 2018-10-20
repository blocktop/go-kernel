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
	proto      spec.Marshalled
	blockQs    *blockQueues
	msgChan    *MessageChannel
	blockchain spec.Blockchain
	consensus  spec.Consensus
	comp       spec.Competition
	genesis    bool
	genNum     uint64
	rootID     int
}

var blk *KernelBlock

func initBlock(c *KernelConfig) {
	b := &KernelBlock{}
	b.proto = c.BlockPrototype
	b.blockchain = c.Blockchain
	b.consensus = c.Consensus
	b.msgChan = NewMessageChannel(b.proto, b.recvHandler)
	b.blockQs = newBlockQueues()

	b.genesis = viper.GetBool("blockchain.genesis")

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
	evalStartTime := time.Now().UnixNano()
	glog.V(3).Infof("%s: running head block evaluator", ktime.String())
	comp := b.consensus.Evaluate()
	evalEndTime := time.Now().UnixNano()
	metrics.setEvalTime(evalEndTime - evalStartTime)

	b.setComp(comp)

	confStartTime := time.Now().UnixNano()
	glog.V(3).Infof("%s: running block confirmer", ktime.String())
	b.consensus.ConfirmBlocks()
	confEndTime := time.Now().UnixNano()
	metrics.setConfBlockTime(confEndTime - confStartTime)

	metrics.setBlockQCount(b.blockQs.count())
}

func (b *KernelBlock) setComp(comp spec.Competition) {
	b.comp = comp
}

func (b *KernelBlock) generate() {
	glog.V(3).Infof("%s: initiating block generation", ktime.String())
	if b.genesis && b.genNum == 0 {
		newBlock := b.blockchain.GenerateGenesis()
		b.genNum = 1
		b.rootID = 1
		b.outputNewLocalBlock(newBlock)
		return
	}

	compBranch := b.evaluateBranches()
	if compBranch == nil {
		glog.V(3).Infof("%s: no competition at block %d", ktime.String(), b.genNum)
		return
	}

	blocks := compBranch.Blocks()
	b.genNum = blocks[0].BlockNumber() + 1

	startTime := time.Now().UnixNano()
	newBlock := b.blockchain.GenerateBlock(blocks, compBranch.RootID())
	endTime := time.Now().UnixNano()

	metrics.setGenBlockTime(endTime - startTime)

	b.outputNewLocalBlock(newBlock)
}

func (b *KernelBlock) evaluateBranches() spec.CompetingBranch {
	if b.comp == nil {
		return nil
	}

	branches := b.comp.Branches()
	curBranch := branches[b.rootID]

	if curBranch != nil {
		// TODO: may want to evaluate other branches as well
		b.consensus.SetConfirmingRoot(b.rootID)
		return curBranch
	}

	var maxHitRate float64 = 0
	var bestRootID int

	for rootID, branch := range branches {
		if branch.HitRate() > maxHitRate && branch.ConsecutiveLocalHits() < 3 { //TODO: make consecutive hits threshold configurable?
			maxHitRate = branch.HitRate()
			bestRootID = rootID
		}
	}

	b.rootID = bestRootID
	b.consensus.SetConfirmingRoot(bestRootID)
	return branches[bestRootID]
}

func (b *KernelBlock) outputNewLocalBlock(newBlock spec.Block) bool {
	netMsg, err := b.makeNetMsg(newBlock)
	if err != nil {
		glog.Error("Failed to make net message from newly generated block")
		return false
	}
	glog.V(3).Infof("%s: generated local block %d:%s", ktime.String(), newBlock.BlockNumber(), newBlock.Hash()[:6])

	// Locally-generated block bypasses the queues, add to consensus immediately.
	res := b.blockchain.AddBlocks([]spec.Block{newBlock}, true)
	if res.Error != nil {
		glog.Errorln("Failed to add locally-generated block to consensus:", res.Error)
	}
	if res.AddedBlock != nil {
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
	res := b.blockchain.AddBlocks(blocks, local)
	if res.Error != nil {
		glog.Errorln("failed to add blocks:", res.Error)
		return
	}
	endTime := time.Now().UnixNano()
	metrics.setAddBlockTime(endTime - startTime)

	if res.AddedBlock != nil {
		netMsg := index[res.AddedBlock.Hash()]
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
