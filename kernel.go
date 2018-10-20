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
	"context"
	"time"

	spec "github.com/blocktop/go-spec"
	"github.com/golang/glog"
)

type Kernel struct {
	name    string
	net     spec.NetworkNode
	started bool
	stop    func()
}

var kernel *Kernel
var initHanlders []func() = make([]func(), 0)

func Init(c *KernelConfig) {

	if !c.valid() {
		panic("all KernelConfig fields are required")
	}

	k := &Kernel{}
	k.name = c.Blockchain.Name()

	kernel = k

	initTime(c.BlockFrequency)
	initMetrics()
	initNet(c.NetworkNode)
	initBlock(c)
	initProc()

	for _, handler := range initHanlders {
		handler()
	}
}

func panicIfUninitialized() {
	if !Initialized() {
		panic("kernel is not initialized")
	}
}

func Initialized() bool {
	return kernel != nil
}

func OnInit(f func()) {
	initHanlders = append(initHanlders, f)
}

func Metrics() *KernelMetrics {
	panicIfUninitialized()
	return metrics
}

func Time() *KernelTime {
	panicIfUninitialized()
	return ktime
}

func Network() *KernelNet {
	panicIfUninitialized()
	return net
}

func Proc() *KernelProc {
	panicIfUninitialized()
	return proc
}

func Block() *KernelBlock {
	panicIfUninitialized()
	return blk
}

func Started() bool {
	return kernel.started
}

func Start(parentCtx context.Context) {
	panicIfUninitialized()
	if kernel.started {
		return
	}

	ctx, cancel := context.WithCancel(parentCtx)
	kernel.stop = cancel

	kernel.started = true

	net.start()

	ktime.up()

	go kernel.runBlockCycle(ctx)
}

func Stop() {
	panicIfUninitialized()
	if !kernel.started {
		return
	}

	net.stop()

	if kernel.stop != nil {
		kernel.stop()
		kernel.stop = nil
	}
	kernel.started = false
}

const (
	blockCycleStateMaint = iota
	blockCycleStateProc
)

func (k *Kernel) runBlockCycle(ctx context.Context) {
	state := blockCycleStateProc

	for {
		select {
		case <-ctx.Done():
			Stop()
			return
		default:
			switch state {
			case blockCycleStateProc:
				ktime.startCycle()
				k.proc()
				state = blockCycleStateMaint
			case blockCycleStateMaint:
				k.maint()
				state = blockCycleStateProc
			}
		}
	}
}

func (k *Kernel) maint() {
	maintStartTime := time.Now().UnixNano()

	glog.V(3).Infoln("------------- maint cycle -------------")
	glog.V(3).Infof("Uptime: %s", ktime.UpTime().String())
	glog.V(3).Infof("Kernel time: %s", ktime.String())

	blk.stop()
	blk.maint()

	net.setMetrics()

	maintEndTime := time.Now().UnixNano()
	metrics.setMaintTime(maintEndTime - maintStartTime)
}

func (k *Kernel) proc() {
	procStartTime := time.Now().UnixNano()

	glog.V(3).Infoln("------------- proc cycle --------------")
	glog.V(3).Infof("Uptime: %s", ktime.UpTime().String())
	glog.V(3).Infof("Kernel time: %s", ktime.String())

	// Anything broadcast during this proc cycle will not be sent
	// Until the cycle is over. This prevents a message created
	// during the cycle from being received during the same cycle
	// e.g. runaway process.
	net.beginProc()

	// Resume processing of blocks received and queued.
	blk.start()

	procTime := metrics.computeProcTime()
	glog.V(3).Infof("%s: computed process time %dms", ktime.String(), procTime/time.Millisecond)

	timer := time.NewTimer(procTime)

	blk.generate()

	// Wait for the block to generate and the timer to expire, which ever comes _last_
	// (blk.generate is not a goroutine).
	<-timer.C

	net.endProc()

	procEndTime := time.Now().UnixNano()
	actualProcTime := procEndTime - procStartTime
	glog.V(3).Infof("%s: actual process time %dms", ktime.String(), actualProcTime/int64(time.Millisecond))

	metrics.setActualProcTime(actualProcTime)
}
