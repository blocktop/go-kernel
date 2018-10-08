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
	"sync"

	push "github.com/blocktop/go-push-components"
	"github.com/blocktop/go-spec"
	"github.com/golang/glog"
)

type KernelNet struct {
	node           spec.NetworkNode
	holdQ          *push.PushBatchQueue
	holdBroadcasts bool
	recvQs         *sync.Map
}

var net *KernelNet

func initNet(node spec.NetworkNode) {
	n := &KernelNet{}
	n.node = node
	n.holdQ = push.NewPushBatchQueue(1, 100000, 1000, n.broadcastHoldDrainWorker)
	n.recvQs = &sync.Map{}
	n.setupMessageReceiver()

	net = n
}

func (n *KernelNet) RegisterMessageChannel(channel *MessageChannel) {
	_, ok := n.recvQs.Load(channel.Protocol.String())
	if ok {
		return
	}
	n.recvQs.Store(channel.Protocol.String(), push.NewPushQueue(1, 100000, func(item interface{}) {
		netMsg := item.(*spec.NetworkMessage)
		channel.ReceiveHandler(netMsg)
	}))
}

func (n *KernelNet) Broadcast(netMsg *spec.NetworkMessage) {
	// TODO: some immediate message integrity checks, and return an error?
	if n.holdBroadcasts {
		n.holdQ.Put(netMsg)
	} else {
		n.priorityBroadcast(netMsg)
	}
}

func (n *KernelNet) priorityBroadcast(netMsg *spec.NetworkMessage) {
	n.node.Broadcast([]*spec.NetworkMessage{netMsg})
}

func (n *KernelNet) start() {
	n.recvQs.Range(func(qid, q interface{}) bool {
		pq := q.(*push.PushQueue)
		pq.Start()
		return true
	})
}

func (n *KernelNet) stop() {
	n.recvQs.Range(func(qid, q interface{}) bool {
		pq := q.(*push.PushQueue)
		pq.Stop()
		return true
	})
}

func (n *KernelNet) PeerID() string {
	return n.node.PeerID()
}

func (n *KernelNet) setMetrics() {
	n.recvQs.Range(func(qid, q interface{}) bool {
		pq := q.(*push.PushQueue)
		metrics.setRecvQCount(qid.(string), pq.Count())
		return true
	})
}

// During kernel proc time, hold broadcast messages in the
// holding queue until proc time is over. This forces
// responses to the broadcast to arrive during the next
// cycle at the earliest and prevents runaway communication
// within one cycle.
func (n *KernelNet) beginProc() {
	glog.V(3).Infof("%s: suspending non-priority message broadcasts", ktime.String())
	n.holdBroadcasts = true
}

func (n *KernelNet) endProc() {
	glog.V(3).Infof("%s: resuming message broadcasts, sending %d held messages", ktime.String(), n.holdQ.Count())
	done := make(chan bool)
	n.holdQ.OnDrained(func() {
		done <- true
	})
	n.holdQ.Drain()
	<-done
	n.holdBroadcasts = false
}

func (n *KernelNet) broadcastHoldDrainWorker(items []interface{}) {
	n.node.Broadcast(castToNetMsg(items))
}

func castToNetMsg(items []interface{}) []*spec.NetworkMessage {
	res := make([]*spec.NetworkMessage, 0)
	for _, item := range items {
		netMsg, ok := item.(*spec.NetworkMessage)
		if !ok {
			glog.Errorln("Broadcast item was not NetworkMessage")
			return nil
		}
		res = append(res, netMsg)
	}
	return res
}

func (n *KernelNet) setupMessageReceiver() {
	n.node.OnMessageReceived(func(netMsg *spec.NetworkMessage) {
		q, ok := n.recvQs.Load(netMsg.Protocol.String())
		if !ok {
			glog.Warningf("Unknown message protocol received %s", netMsg.Protocol.String())
			return
		}
		queue := q.(*push.PushQueue)
		queue.Put(netMsg)
	})
}
