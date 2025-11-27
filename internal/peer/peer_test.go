package peer

import (
	"au_blockchain/internal/account"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

func compareLedgers(l1, l2 *account.Ledger) bool {
	if len(l1.Accounts) != len(l2.Accounts) {
		return false
	}
	for acc, bal := range l1.Accounts {
		if l2.Accounts[acc] != bal {
			return false
		}
	}
	return true
}

func verifyLedgerConsistency(t *testing.T, peers []*Peer) bool {
	ledgers := make([]*account.Ledger, len(peers))
	for i, peer := range peers {
		ledgers[i] = peer.GetLedger()
	}
	return account.VerifyLedgerConsistency(ledgers)
}

func verifyLedgerConsistencyAndComputed(t *testing.T, peers []*Peer, computed *account.Ledger) bool {
	if !verifyLedgerConsistency(t, peers) {
		return false
	}
	if !compareLedgers(peers[0].GetLedger(), computed) {
		return false
	}
	return true
}

func extendTestNetworkLine(t *testing.T, numPeers int, basePort int, entryPort int) []*Peer {
	peers := make([]*Peer, numPeers)

	for i := range numPeers {
		peers[i] = NewPeer(BASE_ADDR, basePort+i)
		var err error
		if i == 0 {
			err = peers[i].Connect(BASE_ADDR, entryPort)
			if err != nil {
				t.Fatalf("First peer failed to connect to entry peer (%d): %v", entryPort, err)
			}
		} else {
			err = peers[i].Connect(BASE_ADDR, basePort+i-1)
			if err != nil {
				t.Fatalf("Peer %d failed to connect to peer %d: %v", i, i-1, err)
			}
		}
		if err != nil {
		}
	}

	return peers
}

func createTestNetworkLine(t *testing.T, numPeers int, basePort int) []*Peer {
	return extendTestNetworkLine(t, numPeers, basePort, basePort)
}

func extendTestNetworkFlower(t *testing.T, numPeers int, basePort int, entryPort int) []*Peer {
	// A network with all peers attached in a flower topology
	peers := make([]*Peer, numPeers)

	for i := range numPeers {
		peers[i] = NewPeer(BASE_ADDR, basePort+i)
		// BECAUSE of the exercise requirements, intenstonally connect to self first
		err := peers[i].Connect(BASE_ADDR, entryPort)
		if err != nil {
			t.Fatalf("Peer %d failed to connect to peer 0: %v", i, err)
		}
	}

	return peers
}

func createTestNetworkFlower(t *testing.T, numPeers int, basePort int) []*Peer {
	return extendTestNetworkFlower(t, numPeers, basePort, basePort)
}

func extendTestNetwork(t *testing.T, numPeers int, basePort int, entryPort int) []*Peer {
	numPeersFlower := numPeers/2 + numPeers%2
	numPeersLine := numPeers - numPeersFlower
	peers_flower := extendTestNetworkFlower(t, numPeersFlower, basePort, entryPort)
	peers_line := extendTestNetworkLine(t, numPeersLine, basePort+numPeersFlower, basePort)
	peers := append(peers_flower, peers_line...)
	return peers
}

func createTestNetwork(t *testing.T, numPeers int, basePort int) []*Peer {
	return extendTestNetwork(t, numPeers, basePort, basePort)
}

func cleanupPeers(peers []*Peer) {
	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(p *Peer) {
			defer wg.Done()
			p.Disconnect()
		}(peer)
	}
	wg.Wait()
	// Give some time for OS to release ports
	time.Sleep(100 * time.Millisecond)
}

func TestCreateNetworkLine(t *testing.T) {
	numPeers := 5
	peers := createTestNetworkLine(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func TestCreateNetworkFlower(t *testing.T) {
	numPeers := 5
	peers := createTestNetworkFlower(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func TestCreateNetworkMixed(t *testing.T) {
	numPeers := 10
	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func TestCreateNetworkRandom(t *testing.T) {
	numPeers := 10
	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func TestCreateBigNetwork(t *testing.T) {
	numPeers := 50
	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

// TODO: TEST set of peers better
func TestPeerList(t *testing.T) {
	numPeers := 10
	time.Sleep(1000 * time.Millisecond)
	peers := createTestNetworkFlower(t, numPeers, 10000)
	defer cleanupPeers(peers)
	time.Sleep(1000 * time.Millisecond)
	if len(peers[0].GetPeers()) != numPeers {
		t.Errorf("Expected %d peers in peer list, got %d", numPeers, len(peers[0].GetPeers()))
	}
}

func TestReuseNetwork(t *testing.T) {
	numPeers := 5
	peers := createTestNetwork(t, numPeers, 10000)
	cleanupPeers(peers)
	peers = createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func TestLedgerConsistency(t *testing.T) {
	numPeers := 10
	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)

	time.Sleep(500 * time.Millisecond)

	peers[0].SendBalance(peers[1].GetEncodedPublicKey(), 500)
	peers[0].SendBalance(peers[1].GetEncodedPublicKey(), 1000)

	time.Sleep(1 * time.Second)

	if !verifyLedgerConsistency(t, peers) {
		t.Errorf("Ledgers are inconsistent after transactions")
	}
}

func TestRandomLedgerConsistency(t *testing.T) {
	numPeers := 5
	numTransactions := 10

	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)

	//computedLedger := account.MakeLedger()

	for i := range numTransactions {
		fromPeerIndex := i % len(peers)
		toPeerIndex := (i + 1) % len(peers)
		peers[fromPeerIndex].SendBalance(
			peers[toPeerIndex].GetEncodedPublicKey(),
			rand.Intn(100),
		)
		//computedLedger.Transaction(tx)
	}

	time.Sleep(1 * time.Second) // Wait for transactions to propagate

	if !verifyLedgerConsistency(t, peers) {
		t.Errorf("Ledgers are inconsistent after random transactions")
	}

	/*
		if !compareLedgers(peers[0].GetLedger(), computedLedger) {
			t.Errorf("Peers' ledgers do not match computed ledger")
		}
	*/
}

func TestDifferentNetworks(t *testing.T) {
	peers_group1 := createTestNetwork(t, 5, 10000)
	defer cleanupPeers(peers_group1)
	peers_group2 := createTestNetwork(t, 5, 20000)
	defer cleanupPeers(peers_group2)
	peers_group1[0].SendBalance(peers_group1[0].GetEncodedPublicKey(), 50)

	time.Sleep(1 * time.Second)
	if len(peers_group2[0].GetLedger().Accounts) != 0 {
		t.Errorf("Expected no accounts in ledger for group2 peer, got: %v",
			peers_group2[0].GetLedger().Accounts)
	}
}

func TestHandinRequirements(t *testing.T) {
	numPeers := 15
	txPerPeer := 10
	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)

	time.Sleep(1 * time.Second)

	computedLedger := account.MakeLedger()
	for i, peer := range peers {
		for range txPerPeer {
			toPeerIndex := (i + 1) % len(peers)
			toPeer := peers[toPeerIndex]
			amount := rand.Intn(100)
			peer.SendBalance(
				toPeer.GetEncodedPublicKey(),
				amount,
			)
			computedLedger.Transaction(account.NewTransaction(
				"tx"+strconv.Itoa(i),
				peer.GetEncodedPublicKey(),
				toPeer.GetEncodedPublicKey(),
				amount,
			))
		}
	}
	time.Sleep(1 * time.Second) // Wait for transactions to propagate
	if !verifyLedgerConsistencyAndComputed(t, peers, computedLedger) {
		t.Errorf("Ledgers are inconsistent or do not match computed ledger")
	}
}

func TestLateJoining(t *testing.T) {
	numPeersGroup1 := 5 // Connected before the transactions are fired
	numPeersGroup2 := 5 // Connects just after the transactions are fired
	peers_group1 := createTestNetwork(t, numPeersGroup1, 10000)
	defer cleanupPeers(peers_group1)

	for i, peer := range peers_group1 {
		toPeerIndex := (i + 1) % len(peers_group1)
		amount := rand.Intn(100)
		peer.SendBalance(
			peers_group1[toPeerIndex].GetEncodedPublicKey(),
			amount,
		)
	}
	time.Sleep(1 * time.Second)
	// When joining this late and no messages are being propagated, there should be consistency
	//  due to handling the messages from the received message history
	peers_group2 := extendTestNetwork(t, numPeersGroup2, 20000, 10000)
	defer cleanupPeers(peers_group2)
	time.Sleep(1 * time.Second)

	peers_merged := append(peers_group1, peers_group2...)

	if !verifyLedgerConsistency(t, peers_group1) {
		t.Errorf("Ledgers are inconsistent in network group 1(even excluding late joining peers)")
	}

	if !verifyLedgerConsistency(t, peers_merged) {
		t.Errorf("Ledgers are inconsistent in merged network(containing late joining peers)")
	}
}

func TestLateJoiningDuringTransactions(t *testing.T) {
	numPeersGroup1 := 5 // Connected before the transactions are fired
	numPeersGroup2 := 5 // Connects just after the transactions are fired
	peers_group1 := createTestNetwork(t, numPeersGroup1, 10000)
	defer cleanupPeers(peers_group1)

	for i, peer := range peers_group1 {
		toPeerIndex := (i + 1) % len(peers_group1)
		amount := rand.Intn(100)
		peer.SendBalance(
			peers_group1[toPeerIndex].GetEncodedPublicKey(),
			amount,
		)
	}
	peers_group2 := extendTestNetwork(t, numPeersGroup2, 20000, 10000)
	defer cleanupPeers(peers_group2)
	time.Sleep(1 * time.Second)

	peers_merged := append(peers_group1, peers_group2...)

	if !verifyLedgerConsistency(t, peers_group1) {
		t.Errorf("Ledgers are inconsistent in network group 1(even excluding late joining peers)")
	}

	if !verifyLedgerConsistency(t, peers_merged) {
		t.Errorf("Ledgers are inconsistent in merged network(containing late joining peers)")
	}
}
