package gossip

import (
	"time"
)

// 探测循环
func (n *Node) probeLoop() {
	probeEvery := n.probeInterval
	if probeEvery <= 0 {
		probeEvery = 500 * time.Millisecond
	}
	ticker := time.NewTicker(probeEvery)
	defer ticker.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.probeOnce(n.ctx)
		}
	}
}

// gossip fanout
func (n *Node) gossipLoop() {
	gossipEvery := n.gossipInterval
	if gossipEvery <= 0 {
		gossipEvery = 1 * time.Second
	}
	ticker := time.NewTicker(gossipEvery)
	defer ticker.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.gossipOnce(n.ctx)
		}
	}
}

// 定期清理循环
func (n *Node) reapLoop() {
	reapEvery := n.probeInterval
	if reapEvery <= 0 {
		reapEvery = 500 * time.Millisecond
	}
	ticker := time.NewTicker(reapEvery)
	defer ticker.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.reapSuspectsAndDeads(n.ctx)
		}
	}
}
