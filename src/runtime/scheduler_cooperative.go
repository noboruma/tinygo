//go:build scheduler.tasks || scheduler.asyncify

package runtime

// This file implements the TinyGo scheduler. This scheduler is a very simple
// cooperative round robin scheduler, with a runqueue that contains a linked
// list of goroutines (tasks) that should be run next, in order of when they
// were added to the queue (first-in, first-out). It also contains a sleep queue
// with sleeping goroutines in order of when they should be re-activated.
//
// The scheduler is used both for the asyncify based scheduler and for the task
// based scheduler. In both cases, the 'internal/task.Task' type is used to represent one
// goroutine.

import (
	"internal/task"
	"runtime/interrupt"
)

// On JavaScript, we can't do a blocking sleep. Instead we have to return and
// queue a new scheduler invocation using setTimeout.
const asyncScheduler = GOOS == "js"

const hasScheduler = true

// Concurrency is not parallelism. While the cooperative scheduler has
// concurrency, it does not have parallelism.
const hasParallelism = false

// Queues used by the scheduler.
var (
	runqueue           task.Queue
	sleepQueue         *task.Task
	sleepQueueBaseTime timeUnit
	timerQueue         *timerNode
)

// deadlock is called when a goroutine cannot proceed any more, but is in theory
// not exited (so deferred calls won't run). This can happen for example in code
// like this, that blocks forever:
//
//	select{}
//
//go:noinline
func deadlock() {
	// call yield without requesting a wakeup
	task.Pause()
	panic("unreachable")
}

// Add this task to the end of the run queue.
func scheduleTask(t *task.Task) {
	runqueue.Push(t)
}

func Gosched() {
	runqueue.Push(task.Current())
	task.Pause()
}

// Add this task to the sleep queue, assuming its state is set to sleeping.
func addSleepTask(t *task.Task, duration timeUnit) {
	if schedulerDebug {
		println("  set sleep:", t, duration)
		if t.Next != nil {
			panic("runtime: addSleepTask: expected next task to be nil")
		}
	}
	t.Data = uint64(duration)
	now := ticks()
	if sleepQueue == nil {
		scheduleLog("  -> sleep new queue")

		// set new base time
		sleepQueueBaseTime = now
	}

	// Add to sleep queue.
	q := &sleepQueue
	for ; *q != nil; q = &(*q).Next {
		if t.Data < (*q).Data {
			// this will finish earlier than the next - insert here
			break
		} else {
			// this will finish later - adjust delay
			t.Data -= (*q).Data
		}
	}
	if *q != nil {
		// cut delay time between this sleep task and the next
		(*q).Data -= t.Data
	}
	t.Next = *q
	*q = t
}

// addTimer adds the given timer node to the timer queue. It must not be in the
// queue already.
// This function is very similar to addSleepTask but for timerQueue instead of
// sleepQueue.
func addTimer(tim *timerNode) {
	mask := interrupt.Disable()

	// Add to timer queue.
	q := &timerQueue
	for ; *q != nil; q = &(*q).next {
		if tim.whenTicks() < (*q).whenTicks() {
			// this will finish earlier than the next - insert here
			break
		}
	}
	tim.next = *q
	*q = tim
	interrupt.Restore(mask)
}

// removeTimer is the implementation of time.stopTimer. It removes a timer from
// the timer queue, returning true if the timer is present in the timer queue.
func removeTimer(tim *timer) bool {
	removedTimer := false
	mask := interrupt.Disable()
	for t := &timerQueue; *t != nil; t = &(*t).next {
		if (*t).timer == tim {
			scheduleLog("removed timer")
			*t = (*t).next
			removedTimer = true
			break
		}
	}
	if !removedTimer {
		scheduleLog("did not remove timer")
	}
	interrupt.Restore(mask)
	return removedTimer
}

func schedulerRunQueue() *task.Queue {
	return &runqueue
}

// Run the scheduler until all tasks have finished.
// There are a few special cases:
//   - When returnAtDeadlock is true, it also returns when there are no more
//     runnable goroutines.
//   - When using the asyncify scheduler, it returns when it has to wait
//     (JavaScript uses setTimeout so the scheduler must return to the JS
//     environment).
func scheduler(returnAtDeadlock bool) {
	// Main scheduler loop.
	var now timeUnit
	for !mainExited {
		scheduleLog("")
		scheduleLog("  schedule")
		if sleepQueue != nil || timerQueue != nil {
			now = ticks()
		}

		// Add tasks that are done sleeping to the end of the runqueue so they
		// will be executed soon.
		if sleepQueue != nil && now-sleepQueueBaseTime >= timeUnit(sleepQueue.Data) {
			t := sleepQueue
			scheduleLogTask("  awake:", t)
			sleepQueueBaseTime += timeUnit(t.Data)
			sleepQueue = t.Next
			t.Next = nil
			runqueue.Push(t)
		}

		// Check for expired timers to trigger.
		if timerQueue != nil && now >= timerQueue.whenTicks() {
			scheduleLog("--- timer awoke")
			delay := ticksToNanoseconds(now - timerQueue.whenTicks())
			// Pop timer from queue.
			tn := timerQueue
			timerQueue = tn.next
			tn.next = nil
			// Run the callback stored in this timer node.
			tn.callback(tn, delay)
		}

		t := runqueue.Pop()
		if t == nil {
			if sleepQueue == nil && timerQueue == nil {
				if returnAtDeadlock {
					return
				}
				if asyncScheduler {
					// JavaScript is treated specially, see below.
					return
				}
				waitForEvents()
				continue
			}

			var timeLeft timeUnit
			if sleepQueue != nil {
				timeLeft = timeUnit(sleepQueue.Data) - (now - sleepQueueBaseTime)
			}
			if timerQueue != nil {
				timeLeftForTimer := timerQueue.whenTicks() - now
				if sleepQueue == nil || timeLeftForTimer < timeLeft {
					timeLeft = timeLeftForTimer
				}
			}

			if schedulerDebug {
				println("  sleeping...", sleepQueue, uint(timeLeft))
				for t := sleepQueue; t != nil; t = t.Next {
					println("    task sleeping:", t, timeUnit(t.Data))
				}
				for tim := timerQueue; tim != nil; tim = tim.next {
					println("---   timer waiting:", tim, tim.whenTicks())
				}
			}
			if timeLeft > 0 {
				sleepTicks(timeLeft)
				if asyncScheduler {
					// The sleepTicks function above only sets a timeout at
					// which point the scheduler will be called again. It does
					// not really sleep. So instead of sleeping, we return and
					// expect to be called again.
					break
				}
			}
			continue
		}

		// Run the given task.
		scheduleLogTask("  run:", t)
		t.Resume()
	}
}

// Pause the current task for a given time.
//
//go:linkname sleep time.Sleep
func sleep(duration int64) {
	if duration <= 0 {
		return
	}

	addSleepTask(task.Current(), nanosecondsToTicks(duration))
	task.Pause()
}

// run is called by the program entry point to execute the go program.
// With a scheduler, init and the main function are invoked in a goroutine before starting the scheduler.
func run() {
	initHeap()
	go func() {
		initAll()
		callMain()
		mainExited = true
	}()
	scheduler(false)
}
