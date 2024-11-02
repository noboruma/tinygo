package runtime

import "internal/task"

const schedulerDebug = false

var mainExited bool

var timerQueue *timerNode

// Simple logging, for debugging.
func scheduleLog(msg string) {
	if schedulerDebug {
		println("---", msg)
	}
}

// Simple logging with a task pointer, for debugging.
func scheduleLogTask(msg string, t *task.Task) {
	if schedulerDebug {
		println("---", msg, t)
	}
}

// Simple logging with a channel and task pointer.
func scheduleLogChan(msg string, ch *channel, t *task.Task) {
	if schedulerDebug {
		println("---", msg, ch, t)
	}
}

func timerQueueAdd(tim *timerNode) {
	q := &timerQueue
	for ; *q != nil; q = &(*q).next {
		if tim.whenTicks() < (*q).whenTicks() {
			// this will finish earlier than the next - insert here
			break
		}
	}
	tim.next = *q
	*q = tim
}

func timerQueueRemove(tim *timer) bool {
	removedTimer := false
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
	return removedTimer
}

// Goexit terminates the currently running goroutine. No other goroutines are affected.
func Goexit() {
	panicOrGoexit(nil, panicGoexit)
}
