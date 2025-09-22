package peer

import (
	"fmt"
	"sync"
	"time"
)

const BASE_ADDR = "127.0.0.1"

func CreateTestNetwork(numPeers int, basePort int) []*Peer {
	return ExtendTestNetwork(numPeers, basePort, basePort)
}

func ExtendTestNetwork(numPeers int, basePort int, entryPort int) []*Peer {
	numPeersFlower := numPeers/2 + numPeers%2
	numPeersLine := numPeers - numPeersFlower
	peersFlower := ExtendTestNetworkFlower(numPeersFlower, basePort, entryPort)
	peersLine := ExtendTestNetworkLine(numPeersLine, basePort+numPeersFlower, basePort)
	peers := append(peersFlower, peersLine...)
	return peers
}

func ExtendTestNetworkFlower(numPeers int, basePort int, entryPort int) []*Peer {
	// A network with all peers attached in a flower topology
	peers := make([]*Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = NewPeer(BASE_ADDR, basePort+i)
		err := peers[i].Connect(BASE_ADDR, entryPort)
		if err != nil {
			fmt.Printf("Warning: Peer %d failed to connect to entry peer: %v\n", i, err)
		}
	}

	return peers
}

func ExtendTestNetworkLine(numPeers int, basePort int, entryPort int) []*Peer {
	peers := make([]*Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = NewPeer(BASE_ADDR, basePort+i)
		var err error
		if i == 0 {
			err = peers[i].Connect(BASE_ADDR, entryPort)
			if err != nil {
				fmt.Printf("Warning: First peer failed to connect to entry peer (%d): %v\n", entryPort, err)
			}
		} else {
			err = peers[i].Connect(BASE_ADDR, basePort+i-1)
			if err != nil {
				fmt.Printf("Warning: Peer %d failed to connect to peer %d: %v\n", i, i-1, err)
			}
		}
	}

	return peers
}

func CleanupPeers(peers []*Peer) {
	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(p *Peer) {
			defer wg.Done()
			p.Disconnect()
		}(peer)
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}
