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
	cycleTime                   movavg.MultiMA
	lastCycleTime               float64
	maintTime                   movavg.MultiMA
	lastMaintTime               float64
	maintTimePercent            movavg.MultiMA
	lastMaintTimePercent        float64
	genBlockTime                movavg.MultiMA
	lastGenBlockTime            float64
	addBlockTime                movavg.MultiMA
	lastAddBlockTime            float64
	confBlockTime               movavg.MultiMA
	lastConfBlockTime           float64
	evalTime                    movavg.MultiMA
	lastEvalTime                float64
	computedProcTime            movavg.MultiMA
	lastComputedProcTime        float64
	computedProcTimePercent     movavg.MultiMA
	lastComputedProcTimePercent float64
	actualProcTime              movavg.MultiMA
	lastActualProcTime          float64
	actualProcTimePercent       movavg.MultiMA
	lastActualProcTimePercent   float64
	blockQCount                 movavg.MultiMA
	lastBlockQCount             float64
	recvQCounts                 *sync.Map
	lastRecvQCounts             *sync.Map
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
	m.recvQCounts = &sync.Map{}     // [protocol]movavg.MultiMA
	m.lastRecvQCounts = &sync.Map{} // [protocol]float64

	metrics = m
}

func (m *KernelMetrics) setCycleTime(duration int64) {
	fdur := float64(duration)
	m.cycleTime.Add(fdur)

	maintPercent := 100 * m.lastMaintTime / fdur
	m.maintTimePercent.Add(maintPercent)

	procPercent := 100 * m.lastActualProcTime / fdur
	m.actualProcTimePercent.Add(procPercent)

	m.lastActualProcTime = float64(duration)
}
func (m *KernelMetrics) CycleTimes() []float64 {
	return m.cycleTime.Avg()
}
func (m *KernelMetrics) CycleTime() float64 {
	return m.lastCycleTime
}

func (m *KernelMetrics) setMaintTime(duration int64) {
	fdur := float64(duration)
	m.maintTime.Add(fdur)
	m.lastMaintTime = fdur
}
func (m *KernelMetrics) MaintTimes() []float64 {
	return m.maintTime.Avg()
}
func (m *KernelMetrics) MaintTime() float64 {
	return m.lastMaintTime
}
func (m *KernelMetrics) MaintTimePercents() []float64 {
	return m.maintTimePercent.Avg()
}
func (m *KernelMetrics) MaintTimePercent() float64 {
	return m.lastMaintTimePercent
}

func (m *KernelMetrics) setGenBlockTime(duration int64) {
	fdur := float64(duration)
	m.genBlockTime.Add(fdur)
	m.lastGenBlockTime = fdur
}
func (m *KernelMetrics) GenBlockTimes() []float64 {
	return m.genBlockTime.Avg()
}
func (m *KernelMetrics) GenBlockTime() float64 {
	return m.lastGenBlockTime
}

func (m *KernelMetrics) setAddBlockTime(duration int64) {
	fdur := float64(duration)
	m.addBlockTime.Add(fdur)
	m.lastAddBlockTime = fdur
}
func (m *KernelMetrics) AddBlockTimes() []float64 {
	return m.addBlockTime.Avg()
}
func (m *KernelMetrics) AddBlockTime() float64 {
	return m.lastAddBlockTime
}

func (m *KernelMetrics) setConfBlockTime(duration int64) {
	fdur := float64(duration)
	m.confBlockTime.Add(fdur)
	m.lastConfBlockTime = fdur
}
func (m *KernelMetrics) ConfBlockTimes() []float64 {
	return m.confBlockTime.Avg()
}
func (m *KernelMetrics) ConfBlockTime() float64 {
	return m.lastConfBlockTime
}

func (m *KernelMetrics) setEvalTime(duration int64) {
	fdur := float64(duration)
	m.evalTime.Add(fdur)
	m.lastEvalTime = fdur
}
func (m *KernelMetrics) EvalTimes() []float64 {
	return m.evalTime.Avg()
}
func (m *KernelMetrics) EvalTime() float64 {
	return m.lastEvalTime
}

func (m *KernelMetrics) setComputedProcTime(duration float64) {
	m.computedProcTime.Add(duration)
	m.lastComputedProcTime = duration

	durPercent := duration * ktime.BlockFrequency() * 100 / float64(time.Second)
	m.computedProcTimePercent.Add(durPercent)
	m.lastComputedProcTimePercent = durPercent
}
func (m *KernelMetrics) ComputedProcTimes() []float64 {
	return m.computedProcTime.Avg()
}
func (m *KernelMetrics) ComputedProcTime() float64 {
	return m.lastComputedProcTime
}
func (m *KernelMetrics) ComputedProcTimePercents() []float64 {
	return m.computedProcTimePercent.Avg()
}
func (m *KernelMetrics) ComputedProcTimePercent() float64 {
	return m.lastComputedProcTimePercent
}

func (m *KernelMetrics) setActualProcTime(duration int64) {
	fdur := float64(duration)
	m.actualProcTime.Add(fdur)
	m.lastActualProcTime = fdur
}
func (m *KernelMetrics) ActualProcTimes() []float64 {
	return m.actualProcTime.Avg()
}
func (m *KernelMetrics) ActualProcTime() float64 {
	return m.lastActualProcTime
}
func (m *KernelMetrics) ActualProcTimePercents() []float64 {
	return m.actualProcTimePercent.Avg()
}
func (m *KernelMetrics) ActualProcTimePercent() float64 {
	return m.lastActualProcTimePercent
}

func (m *KernelMetrics) setBlockQCount(count int) {
	fcount := float64(count)
	m.blockQCount.Add(fcount)
	m.lastBlockQCount = fcount
}
func (m *KernelMetrics) BlockQCounts() []float64 {
	return m.blockQCount.Avg()
}
func (m *KernelMetrics) BlockQCount() float64 {
	return m.lastBlockQCount
}

func (m *KernelMetrics) setRecvQCount(name string, count int) {
	fcount := float64(count)
	m.getRecvQ(name).Add(fcount)
	m.lastRecvQCounts.Store(name, fcount)
}
func (m *KernelMetrics) RecvQCounts(name string) []float64 {
	return m.getRecvQ(name).Avg()
}
func (m *KernelMetrics) RecvQCount(name string) float64 {
	fcount, ok := m.lastRecvQCounts.Load(name)
	if !ok {
		return 0
	}
	return fcount.(float64)
}
func (m *KernelMetrics) RecvQCountsMap() map[string][]float64 {
	res := make(map[string][]float64)
	m.recvQCounts.Range(func(n, s interface{}) bool {
		res[n.(string)] = s.(movavg.MultiMA).Avg()
		return true
	})
	return res
}
func (m *KernelMetrics) RecvQCountMap() map[string]float64 {
	res := make(map[string]float64)
	m.lastRecvQCounts.Range(func(n, s interface{}) bool {
		res[n.(string)] = s.(float64)
		return true
	})
	return res
}

func (m *KernelMetrics) getRecvQ(name string) movavg.MultiMA {
	set, _ := m.recvQCounts.LoadOrStore(name, movavg.NewMultiSMA(SMAWindows))
	return set.(movavg.MultiMA)
}

func (m *KernelMetrics) computeProcTime() time.Duration {
	maintAvg := m.MaintTimes()[0]
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
	rqcs := m.RecvQCountsMap()
	for n, rqc := range rqcs {
		b.WriteString(fmt.Sprintf("  %s: %v\n", n, rqc))
	}
	b.WriteString("--- Cycles ---\n")
	b.WriteString(fmt.Sprintf("Cycle number: %d\n", ktime.CycleNumber()))
	b.WriteString(fmt.Sprintf("Block number: %d\n", blk.BlockNumber()))
	b.WriteString(fmt.Sprintf("Configured cycle time (block interval): %s\n", ktime.BlockInterval().String()))
	b.WriteString(fmt.Sprintf("Actual cycle time (ns): %v\n", m.CycleTimes()))
	b.WriteString("--- Process Timeslice ---\n")
	b.WriteString(fmt.Sprintf("Process timeslice time (ns): %v\n", m.ActualProcTimes()))
	b.WriteString(fmt.Sprintf("Process timeslice %% of block interval: %v\n", m.ActualProcTimePercents()))
	b.WriteString(fmt.Sprintf("Scheduled process timeslice time (ns): %v\n", m.ComputedProcTimes()))
	b.WriteString(fmt.Sprintf("Scheduled proccess timeslice %% of block interval: %v\n", m.ComputedProcTimePercents()))
	b.WriteString(fmt.Sprintf("Block generation time (ns): %v\n", m.GenBlockTimes()))
	b.WriteString(fmt.Sprintf("Block add performance (ns): %v\n", m.AddBlockTimes()))

	b.WriteString("--- Maintenance Timeslice ---\n")
	b.WriteString(fmt.Sprintf("Maintenance timesclice time (ns): %v\n", m.MaintTimes()))
	b.WriteString(fmt.Sprintf("Maintenance timesclice %% of block interval: %v\n", m.MaintTimePercents()))
	b.WriteString(fmt.Sprintf("Block confirmation time (ns): %v\n", m.ConfBlockTimes()))
	b.WriteString(fmt.Sprintf("Head block evaluation time (ns): %v\n", m.ConfBlockTimes()))

	return b.String()
}

type KernelMetricsJSON struct {
	KernelTime                        string               `json:"kernelTime"`
	Uptime                            time.Duration        `json:"uptime"`
	MovingAverageWindows              []int                `json:"movingAverageWindows"`
	BlockQueueCount                   float64              `json:"blockQueueCount"`
	BlockQueueCounts                  []float64            `json:"blockQueueCounts"`
	ReceiveQueueCount                 map[string]float64   `json:"receiveQueueCount"`
	ReceiveQueueCounts                map[string][]float64 `json:"receiveQueueCounts"`
	CycleNumber                       uint64               `json:"cycleNumber,string"`
	ConfiguredCycleTime               time.Duration        `json:"configuredCycleTime"`
	ConfiguredBlockFrequency          float64              `json:"configuredBlockFrequency"`
	ActualCycleTime                   float64              `json:"actualCycleTime"`
	ActualCycleTimes                  []float64            `json:"actualCycleTimes"`
	ProcessTimeslice                  float64              `json:"processTimeslice"`
	ProcessTimeslices                 []float64            `json:"processTimeslices"`
	ProcessTimeslicePercent           float64              `json:"processTimeslicePercent"`
	ProcessTimeslicePercents          []float64            `json:"processTimeslicePercents"`
	ScheduledProcessTimeslice         float64              `json:"scheduleProcessTimeslice"`
	ScheduledProcessTimeslices        []float64            `json:"scheduledProcessTimeslices"`
	ScheduledProcessTimeslicePercent  float64              `json:"scheduledProcessTimeslicePercent"`
	ScheduledProcessTimeslicePercents []float64            `json:"scheduledProcessTimeslicePercents"`
	BlockGenerationNumber             uint64               `json:"blockGenerationNumber,string"`
	BlockGenerationTime               float64              `json:"blockGenerationTime"`
	BlockGenerationTimes              []float64            `json:"blockGenerationTimes"`
	BlockAddPerformance               float64              `json:"blockAddPerformance"`
	BlockAddPerformances              []float64            `json:"blockAddPerformances"`
	MaintenanceTimeslice              float64              `json:"maintenanceTimeslice"`
	MaintenanceTimeslices             []float64            `json:"maintenanceTimeslices"`
	MaintenanceTimeslicePercent       float64              `json:"maintenanceTimeslicePercent"`
	MaintenanceTimeslicePercents      []float64            `json:"maintenanceTimeslicePercents"`
	BlockConfirmationTime             float64              `json:"blockConfirmationTime"`
	BlockConfirmationTimes            []float64            `json:"blockConfirmationTimes"`
	HeadBlockEvaluationTime           float64              `json:"headBlockEvaluationTime"`
	HeadBlockEvaluationTimes          []float64            `json:"headBlockEvaluationTimes"`
}

func (m *KernelMetrics) JSON() (string, error) {
	mj := &KernelMetricsJSON{
		KernelTime:                        ktime.String(),
		Uptime:                            ktime.UpTime(),
		MovingAverageWindows:              SMAWindows,
		BlockQueueCounts:                  m.BlockQCounts(),
		BlockQueueCount:                   m.BlockQCount(),
		ReceiveQueueCounts:                m.RecvQCountsMap(),
		ReceiveQueueCount:                 m.RecvQCountMap(),
		CycleNumber:                       ktime.CycleNumber(),
		ConfiguredCycleTime:               ktime.BlockInterval(),
		ConfiguredBlockFrequency:          ktime.BlockFrequency(),
		ActualCycleTime:                   m.CycleTime(),
		ActualCycleTimes:                  m.CycleTimes(),
		ProcessTimeslice:                  m.ActualProcTime(),
		ProcessTimeslices:                 m.ActualProcTimes(),
		ProcessTimeslicePercent:           m.ActualProcTimePercent(),
		ProcessTimeslicePercents:          m.ActualProcTimePercents(),
		ScheduledProcessTimeslice:         m.ComputedProcTime(),
		ScheduledProcessTimeslices:        m.ComputedProcTimes(),
		ScheduledProcessTimeslicePercent:  m.ComputedProcTimePercent(),
		ScheduledProcessTimeslicePercents: m.ComputedProcTimePercents(),
		BlockGenerationNumber:             blk.BlockNumber(),
		BlockGenerationTime:               m.GenBlockTime(),
		BlockGenerationTimes:              m.GenBlockTimes(),
		BlockAddPerformance:               m.AddBlockTime(),
		BlockAddPerformances:              m.AddBlockTimes(),
		MaintenanceTimeslice:              m.MaintTime(),
		MaintenanceTimeslices:             m.MaintTimes(),
		MaintenanceTimeslicePercent:       m.MaintTimePercent(),
		MaintenanceTimeslicePercents:      m.MaintTimePercents(),
		BlockConfirmationTime:             m.ConfBlockTime(),
		BlockConfirmationTimes:            m.ConfBlockTimes(),
		HeadBlockEvaluationTime:           m.EvalTime(),
		HeadBlockEvaluationTimes:          m.EvalTimes()}

	byts, err := json.Marshal(mj)
	if err != nil {
		return "", err
	}
	return string(byts), nil
}
