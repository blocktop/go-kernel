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
	"sort"
	"sync"

	push "github.com/blocktop/go-push-components"
	spec "github.com/blocktop/go-spec"
)

type blockQueue struct {
	parentID    string
	blockNumber uint64
	blockQ      *push.PushBatchQueue
}

type blockQueues struct {
	queues           *sync.Map // [parentID]*blockQueue
	blockNumberIndex *sync.Map // [blocknumber]mape[parentID]parentID
	started          bool
}

type blockQueueItem struct {
	block  spec.Block
	netMsg *spec.NetworkMessage
}

func newBlockQueues() *blockQueues {
	qs := &blockQueues{}
	qs.queues = &sync.Map{}
	qs.blockNumberIndex = &sync.Map{}
	return qs
}

func (qs *blockQueues) start() {
	qs.started = true

	blockNumbers := make([]uint64, 0)
	qs.blockNumberIndex.Range(func(bn, pids interface{}) bool {
		blockNumbers = append(blockNumbers, bn.(uint64))
		return true
	})

	sort.Slice(blockNumbers, func(i, j int) bool { return blockNumbers[i] < blockNumbers[j] })

	for _, blockNumber := range blockNumbers {
		if !qs.started {
			return
		}

		pids, ok := qs.blockNumberIndex.Load(blockNumber)
		if !ok {
			continue
		}
		parentIDs := pids.(map[string]string)
		for parentID := range parentIDs {
			if !qs.started {
				return
			}

			q, ok := qs.queues.Load(parentID)
			if !ok {
				continue
			}
			bq := q.(*blockQueue)
			done := make(chan bool)
			bq.blockQ.OnDrained(func() {
				done <- true
			})
			bq.blockQ.Drain()
			<-done
			bq.blockQ.Stop()
		}
	}
}

func (qs *blockQueues) stop() {
	qs.started = false
	qs.queues.Range(func(pid, q interface{}) bool {
		bq := q.(*blockQueue)
		bq.blockQ.Stop()
		return true
	})
}

func (qs *blockQueues) delete(parentID string) {
	qs.queues.Delete(parentID)
	var deleteIndex int64 = -1
	qs.blockNumberIndex.Range(func(bn, pids interface{}) bool {
		parentIDs := pids.(map[string]string)
		for pid := range parentIDs {
			if pid == parentID {
				delete(parentIDs, parentID)
				deleteIndex = int64(bn.(uint64))
				return false
			}
		}
		return true
	})
	if deleteIndex > -1 {
		qs.blockNumberIndex.Delete(uint64(deleteIndex))
	}
}

func (qs *blockQueues) count() int {
	c := 0
	qs.queues.Range(func(pid, q interface{}) bool {
		c += q.(*blockQueue).blockQ.Count()
		return true
	})
	return c
}

func (qs *blockQueues) put(block spec.Block, netMsg *spec.NetworkMessage) {
	parentID := block.ParentHash()
	q, ok := qs.queues.Load(parentID)
	if !ok {
		q = newBlockQueue(parentID, block.BlockNumber())
		qs.queues.Store(parentID, q)
		pids, ok := qs.blockNumberIndex.Load(block.BlockNumber())
		if !ok {
			pids = make(map[string]string)
			qs.blockNumberIndex.Store(block.BlockNumber(), pids)
		}
		parentIDs := pids.(map[string]string)
		parentIDs[parentID] = parentID
	}
	bq := q.(*blockQueue)
	bqi := &blockQueueItem{block, netMsg}
	bq.blockQ.Put(bqi)
}

func newBlockQueue(parentID string, blockNumber uint64) *blockQueue {
	q := &blockQueue{parentID: parentID, blockNumber: blockNumber}
	q.blockQ = push.NewPushBatchQueue(1, 100000, 100, func(items []interface{}) {
		blk.blockBatchWorker(castToBlockQueueItems(items), false)
	})
	return q
}

func castToBlockQueueItems(items []interface{}) []*blockQueueItem {
	res := make([]*blockQueueItem, len(items))
	for i, item := range items {
		res[i] = item.(*blockQueueItem)
	}
	return res
}