package consensus

import (
	"github.com/hashicorp/raft"
)

type Scheduler struct {
	raft *raft.Raft

	loopFunc func(chan struct{})
	done     chan struct{}
}

func newScheduler(raft *raft.Raft, loopFunc func(chan struct{})) *Scheduler {
	ri := &Scheduler{
		raft:     raft,
		loopFunc: loopFunc,
		done:     make(chan struct{}),
	}
	go ri.init()
	return ri
}

func (s *Scheduler) init() {
	sig := make(chan struct{})
	for {
		select {
		case isLeader := <-s.raft.LeaderCh():
			if isLeader {
				go s.loopFunc(sig)
			} else {
				sig <- struct{}{}
			}
		case <-s.done:
			close(sig)
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.done)
}
