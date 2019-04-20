package nicerx

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/chzchzchz/nicerx/radio"
)

type Task interface {
	Name() string
	Step(ctx context.Context) error
	Band() radio.FreqBand
}

type idleTask struct{}

func (it *idleTask) Name() string { return "idle" }
func (it *idleTask) Step(ctx context.Context) error {
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
	}
	return ctx.Err()
}

func (it *idleTask) Band() radio.FreqBand { return radio.FreqBand{} }

func newIdleTask() Task { return &idleTask{} }

type TaskId int

type ScheduledTask struct {
	Task
	Id            TaskId
	Priority      int
	startTime     time.Time
	stopTime      time.Time
	totalDuration time.Duration
	mu            sync.RWMutex
}

func (st ScheduledTask) Duration() time.Duration {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.startTime.IsZero() || st.stopTime.After(st.startTime) {
		return st.totalDuration
	}
	return (st.totalDuration + time.Since(st.startTime)).Truncate(time.Millisecond)
}

type TaskQueue struct {
	Running  map[TaskId]*ScheduledTask
	Paused   map[TaskId]*ScheduledTask
	AllTasks map[TaskId]*ScheduledTask
	nextId   TaskId
	mu       sync.RWMutex
}

func (tq *TaskQueue) Add(t Task) (tid TaskId) {
	tq.mu.Lock()
	tid = tq.nextId
	st := &ScheduledTask{Task: t, Id: tid}
	tq.Running[tid] = st
	tq.AllTasks[tid] = st
	tq.nextId++
	tq.mu.Unlock()
	return tid
}

func NewTaskQueue() *TaskQueue {
	tq := &TaskQueue{
		Running:  make(map[TaskId]*ScheduledTask),
		Paused:   make(map[TaskId]*ScheduledTask),
		AllTasks: make(map[TaskId]*ScheduledTask),
	}
	tq.Prioritize(tq.Add(newIdleTask()), -100)
	return tq
}

func (tq *TaskQueue) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		t := tq.nextTask()
		if t == nil {
			return nil
		}
		t.mu.Lock()
		t.startTime = time.Now()
		t.mu.Unlock()
		err := t.Step(ctx)
		if err == io.EOF {
			tq.Stop(t.Id)
		}
		t.mu.Lock()
		t.stopTime = time.Now()
		t.totalDuration += t.stopTime.Sub(t.startTime)
		t.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

func (tq *TaskQueue) nextTask() *ScheduledTask {
	var bestTask *ScheduledTask
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	for _, t := range tq.Running {
		if bestTask == nil {
			bestTask = t
		} else if bestTask.Priority < t.Priority {
			bestTask = t
		}
	}
	return bestTask
}

func (tq *TaskQueue) Prioritize(id TaskId, pr int) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if t, ok := tq.AllTasks[id]; ok {
		t.Priority = pr
	}
}

func (tq *TaskQueue) Resume(id TaskId) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if t, ok := tq.Paused[id]; ok {
		delete(tq.Paused, id)
		tq.Running[id] = t
	}
}

func (tq *TaskQueue) Pause(id TaskId) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if t, ok := tq.Running[id]; ok {
		delete(tq.Running, id)
		tq.Paused[id] = t
	}
}

func (tq *TaskQueue) Stop(id TaskId) {
	tq.mu.Lock()
	delete(tq.Running, id)
	delete(tq.Paused, id)
	delete(tq.AllTasks, id)
	tq.mu.Unlock()
}

func (tq *TaskQueue) Freqs(name string) map[float64]radio.FreqBand {
	ret := make(map[float64]radio.FreqBand)
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	for _, v := range tq.AllTasks {
		if v.Name() == name {
			fb := v.Band()
			ret[fb.Center] = fb
		}
	}
	return ret
}
