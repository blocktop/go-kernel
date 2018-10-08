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
	"sync"
)

type KernelProc struct {
	procs *sync.Map
	running bool
}

type kproc struct {
	process Process
	pid uint
	running bool
	cancel func()
}

type Process interface {
	Name() string
	Namespace() string
	Start(context.Context)
	Stopping()
}

var proc *KernelProc
var procID uint

func initProc() {
	p := &KernelProc{}
	p.procs = &sync.Map{}
	proc = p
}

func (p *KernelProc) IsScheduled(pid uint) bool {
	_, ok := p.procs.Load(pid)
	return ok
}

func (p *KernelProc) Schedule(process Process) (pid uint) {
	pid = procID
	procID++
	kp := &kproc{pid: pid, process: process}
	p.procs.Store(pid, kp)
	return pid
}

func (p *KernelProc) Kill(pid uint) {
	pr, ok := p.procs.Load(pid)
	if ok {
		kp := pr.(*kproc)
		kp.stop()
		p.procs.Delete(pid)		
	}
}

func (p *KernelProc) run(ctx context.Context) {
	p.procs.Range(func(id, pr interface{}) bool {
		kp := pr.(*kproc)
		kp.run(ctx)
		return true
	})
}

func (p *KernelProc) stop() {
	p.procs.Range(func(id, pr interface{}) bool {
		kp := pr.(*kproc) 
		kp.stop()
		return true
	})
}

func (kp *kproc) run(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	kp.cancel = cancel
	kp.running = true
	done := make(chan bool)
	go func() {
		kp.process.Start(ctx)
		done <- true
	}()
	<-done
	kp.running = false
	kp.cancel = nil
}

func (kp *kproc) stop() {
	if kp.running {
		kp.process.Stopping()
		kp.cancel()
		kp.running = false
		kp.cancel = nil
	}
}