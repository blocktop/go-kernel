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
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/blocktop/movavg"
	"github.com/fatih/color"
	"github.com/golang/glog"
)

type KernelMetrics struct {
	cycleTime               movavg.MultiMA
	maintTime               movavg.MultiMA
	maintTimePercent        movavg.MultiMA
	genBlockTime            movavg.MultiMA
	addBlockTime            movavg.MultiMA
	confBlockTime           movavg.MultiMA
	evalTime                movavg.MultiMA
	computedProcTime        movavg.MultiMA
	computedProcTimePercent movavg.MultiMA
	actualProcTime          movavg.MultiMA
	actualProcTimePercent   movavg.MultiMA
	blockQCount             movavg.MultiMA
	recvQCounts             *sync.Map
	lastMaintTime           int64
	lastProcTime            int64
}

var metrics *KernelMetrics
var SMAWindows = []int{10, 100, 1000, 10000, 100000, 1000000}

func initMetrics() {
	m := &KernelMetrics{}
	m.cycleTime = movavg.NewMultiSMA(SMAWindows)
	m.maintTime = movavg.NewMultiSMA(SMAWindows)
	m.maintTimePercent = movavg.NewMultiSMA(SMAWindows)
	m.genBlockTime = movavg.NewMultiSMA(SMAWindows)
	m.addBlockTime = movavg.NewMultiSMA(SMAWindows)
	m.confBlockTime = movavg.NewMultiSMA(SMAWindows)
	m.evalTime = movavg.NewMultiSMA(SMAWindows)
	m.computedProcTime = movavg.NewMultiSMA(SMAWindows)
	m.computedProcTimePercent = movavg.NewMultiSMA(SMAWindows)
	m.actualProcTimePercent = movavg.NewMultiSMA(SMAWindows)
	m.actualProcTime = movavg.NewMultiSMA(SMAWindows)
	m.blockQCount = movavg.NewMultiSMA(SMAWindows) 
	m.recvQCounts = &sync.Map{} // [protocol]movavg.MultiMA

	metrics = m
}

func (m *KernelMetrics) setCycleTime(duration int64) {
	fdur := float64(duration)
	m.cycleTime.Add(fdur)

	maintPercent := 100 * float64(m.lastMaintTime)/fdur 
	m.maintTimePercent.Add(maintPercent)

	procPercent := 100 * float64(m.lastProcTime)/fdur
	m.actualProcTimePercent.Add(procPercent)
}
func (m *KernelMetrics) CycleTime() []float64 {
	return m.cycleTime.Avg()
}

func (m *KernelMetrics) setMaintTime(duration int64) {
	m.maintTime.Add(float64(duration))
	m.lastMaintTime = duration
}
func (m *KernelMetrics) MaintTime() []float64 {
	return m.maintTime.Avg()
}
func (m *KernelMetrics) MaintTimePercent() []float64 {
	return m.maintTimePercent.Avg()
}

func (m *KernelMetrics) setGenBlockTime(duration int64) {
	m.genBlockTime.Add(float64(duration))
}
func (m *KernelMetrics) GenBlockTime() []float64 {
	return m.genBlockTime.Avg()
}

func (m *KernelMetrics) setAddBlockTime(duration int64) {
	m.addBlockTime.Add(float64(duration))
}
func (m *KernelMetrics) AddBlockTime() []float64 {
	return m.addBlockTime.Avg()
}

func (m *KernelMetrics) setConfBlockTime(duration int64) {
	m.confBlockTime.Add(float64(duration))
}
func (m *KernelMetrics) ConfBlockTime() []float64 {
	return m.confBlockTime.Avg()
}

func (m *KernelMetrics) setEvalTime(duration int64) {
	m.evalTime.Add(float64(duration))
}
func (m *KernelMetrics) EvalTime() []float64 {
	return m.evalTime.Avg()
}

func (m *KernelMetrics) setComputedProcTime(duration float64) {
	m.computedProcTime.Add(duration)

	durPercent := duration * ktime.BlockFrequency() * 100 / float64(time.Second)
	m.computedProcTimePercent.Add(durPercent)
}
func (m *KernelMetrics) ComputedProcTime() []float64 {
	return m.computedProcTime.Avg()
}
func (m *KernelMetrics) ComputedProcTimePercent() []float64 {
	return m.computedProcTimePercent.Avg()
}

func (m *KernelMetrics) setActualProcTime(duration int64) {
	fdur := float64(duration)
	m.actualProcTime.Add(fdur)
	m.lastProcTime = duration
}
func (m *KernelMetrics) ActualProcTime() []float64 {
	return m.actualProcTime.Avg()
}
func (m *KernelMetrics) ActualProcTimePercent() []float64 {
	return m.actualProcTimePercent.Avg()
}

func (m *KernelMetrics) setBlockQCount(count int) {
	m.blockQCount.Add(float64(count))
}
func (m *KernelMetrics) BlockQCount() []float64 {
	return m.blockQCount.Avg()
}

func (m *KernelMetrics) setRecvQCount(name string, count int) {
	m.getRecvQ(name).Add(float64(count))
}
func (m *KernelMetrics) RecvQCount(name string) []float64 {
	return m.getRecvQ(name).Avg()
}
func (m *KernelMetrics) RecvQCounts() map[string][]float64 {
	res := make(map[string][]float64)
	m.recvQCounts.Range(func(n, s interface{}) bool {
		res[n.(string)] = s.(movavg.MultiMA).Avg()
		return true
	})
	return res
}

func (m *KernelMetrics) getRecvQ(name string) movavg.MultiMA {
	set, _ := m.recvQCounts.LoadOrStore(name, movavg.NewMultiSMA(SMAWindows))
	return set.(movavg.MultiMA)
}

func (m *KernelMetrics) computeProcTime() time.Duration {
	maintAvg := m.MaintTime()[0]
	procTime := float64(time.Second)/float64(ktime.BlockFrequency()) - maintAvg
	m.setComputedProcTime(procTime)

	if procTime < 0 {
		glog.Errorln(color.HiRedString("%s: proc time overrun by %fns", ktime.String(), procTime*-1))
		return 0
	}
	return time.Duration(int64(procTime))
}

func (m *KernelMetrics) String() string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Kernel time (cycle.nanos): %s\n", ktime.String()))
	b.WriteString(fmt.Sprintf("Kernel uptime (duration): %s\n", ktime.UpTime().String()))
	b.WriteString(fmt.Sprintf("Moving average windows (num blocks): %v\n", SMAWindows))
	b.WriteString(fmt.Sprintf("Block queue count: %v\n", m.BlockQCount()))
	b.WriteString("Receive queue count:\n")
	rqcs := m.RecvQCounts()
	for n, rqc := range rqcs {
		b.WriteString(fmt.Sprintf("  %s: %v\n", n, rqc))
	}
	b.WriteString("--- Cycles ---\n")
	b.WriteString(fmt.Sprintf("Cycle number: %d\n", ktime.CycleNumber()))
	b.WriteString(fmt.Sprintf("Configured cycle time (block interval): %s\n", ktime.BlockInterval().String()))
	b.WriteString(fmt.Sprintf("Actual cycle time (ns): %v\n", m.CycleTime()))
	b.WriteString("--- Process Timeslice ---\n")
	b.WriteString(fmt.Sprintf("Process timeslice time (ns): %v\n", m.ActualProcTime()))
	b.WriteString(fmt.Sprintf("Process timeslice %% of block interval: %v\n", m.ActualProcTimePercent()))
	b.WriteString(fmt.Sprintf("Scheduled process timeslice time (ns): %v\n", m.ComputedProcTime()))
	b.WriteString(fmt.Sprintf("Scheduled proccess timeslice %% of block interval: %v\n", m.ComputedProcTimePercent()))
	b.WriteString(fmt.Sprintf("Block generation time (ns): %v\n", m.GenBlockTime()))
	b.WriteString(fmt.Sprintf("Block add performance (ns): %v\n", m.AddBlockTime()))
	
	b.WriteString("--- Maintenance Timeslice ---\n")
	b.WriteString(fmt.Sprintf("Maintenance timesclice time (ns): %v\n", m.MaintTime()))
	b.WriteString(fmt.Sprintf("Maintenance timesclice %% of block interval: %v\n", m.MaintTimePercent()))
	b.WriteString(fmt.Sprintf("Block confirmation time (ns): %v\n", m.ConfBlockTime()))
	b.WriteString(fmt.Sprintf("Head block evaluation time (ns): %v\n", m.ConfBlockTime()))

	return b.String()
}


type KernelMetricsJSON struct {
	KernelTime string
	Uptime time.Duration
	MovingAverageWindows []int
	BlockQueueCounts []float64
	ReceiveQueueCounts map[string][]float64
	CycleNumber uint64 `json:",string"`
	ConfiguredCycleTime time.Duration
	ActualCycleTime []float64
	ProcessTimeslice []float64
	ProcessTimeslicePercent []float64
	ScheduledProcessTimeslice []float64
	ScheduledProcessTimeslicePercent []float64
	BlockGenerationTime []float64
	BlockAddPerformance []float64
	MaintenanceTimeslice []float64
	MaintenanceTimeslicePercent []float64
	BlockConfirmationTime []float64
	HeadBlockEvaluationTime []float64
}

func (m *KernelMetrics) JSON() (string, error) {
	mj := &KernelMetricsJSON{
		KernelTime: ktime.String(),
		Uptime: ktime.UpTime(),
		MovingAverageWindows: SMAWindows,
		BlockQueueCounts: m.BlockQCount(),
		ReceiveQueueCounts: m.RecvQCounts(),
		CycleNumber: ktime.CycleNumber(),
		ConfiguredCycleTime: ktime.BlockInterval(),
		ActualCycleTime: m.CycleTime(),
		ProcessTimeslice: m.ActualProcTime(),
		ProcessTimeslicePercent: m.ActualProcTimePercent(),
		ScheduledProcessTimeslice: m.ComputedProcTime(),
		ScheduledProcessTimeslicePercent: m.ComputedProcTimePercent(),
		BlockGenerationTime: m.GenBlockTime(),
		BlockAddPerformance: m.AddBlockTime(),
		MaintenanceTimeslice: m.MaintTime(),
		MaintenanceTimeslicePercent: m.MaintTimePercent(),
		BlockConfirmationTime: m.ConfBlockTime(),
		HeadBlockEvaluationTime: m.EvalTime()}

	byts, err := json.Marshal(mj)
	if err != nil {
		return "", err
	}
	return string(byts), nil
}