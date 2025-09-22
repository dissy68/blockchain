package peer

import (
	"ledger/internal/account"
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
	peers := createTestNetworkFlower(t, numPeers, 10000)
	defer cleanupPeers(peers)
	time.Sleep(200 * time.Millisecond)
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

	tx1 := account.NewTransaction("tx1", "Alice", "Bob", 50)
	tx2 := account.NewTransaction("tx2", "Bob", "Alice", 50)
	tx3 := account.NewTransaction("tx3", "Alice", "Carol", 100)
	tx4 := account.NewTransaction("tx4", "Alice", "Carol", 100)
	tx5 := account.NewTransaction("tx5", "Carol", "Bob", 20)

	go peers[0].FloodTransaction(tx1)
	go peers[0].FloodTransaction(tx2)
	go peers[1].FloodTransaction(tx3)
	go peers[2].FloodTransaction(tx4)
	go peers[1].FloodTransaction(tx5)

	time.Sleep(1 * time.Second) // Wait for transactions to propagate, BAD PRACTICE

	if !verifyLedgerConsistency(t, peers) {
		t.Errorf("Ledgers are inconsistent after transactions")
	}
}

func TestRandomLedgerConsistency(t *testing.T) {
	numPeers := 5
	numTransactions := 10
	names := []string{"Alice", "Bob", "Carol", "Dave", "Eve"}

	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)

	computedLedger := account.MakeLedger()

	for i := range numTransactions {
		randFrom := names[i%len(names)]
		randTo := names[(i+1)%len(names)]
		randAmount := (i + 1) * 10
		tx := account.NewTransaction("tx"+strconv.Itoa(i), randFrom, randTo, randAmount)
		peerIndex := i % len(peers)
		peers[peerIndex].FloodTransaction(tx)
		computedLedger.Transaction(tx)
	}

	time.Sleep(1 * time.Second) // Wait for transactions to propagate

	if !verifyLedgerConsistency(t, peers) {
		t.Errorf("Ledgers are inconsistent after random transactions")
	}

	if !compareLedgers(peers[0].GetLedger(), computedLedger) {
		t.Errorf("Peers' ledgers do not match computed ledger")
	}
}

func TestDifferentNetworks(t *testing.T) {
	peers_group1 := createTestNetwork(t, 5, 10000)
	defer cleanupPeers(peers_group1)
	peers_group2 := createTestNetwork(t, 5, 20000)
	defer cleanupPeers(peers_group2)
	tx := account.NewTransaction("tx1", "Alice", "Bob", 50)
	peers_group1[0].FloodTransaction(tx)

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
	computedLedger := account.MakeLedger()
	for i, peer := range peers {
		for j := range txPerPeer {
			tx := account.NewTransaction(
				"tx"+strconv.Itoa(i),
				"account"+strconv.Itoa(j%5),
				"account"+strconv.Itoa((j+1)%5),
				rand.Intn(100),
			)
			computedLedger.Transaction(tx)
			peer.FloodTransaction(tx)
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

	base_tx := account.NewTransaction("tx", "Alice", "Bob", 100)

	for i, peer := range peers_group1 {
		tx := *base_tx
		tx.ID = tx.ID + strconv.Itoa(i)
		peer.FloodTransaction(&tx)
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

	base_tx := account.NewTransaction("tx", "Alice", "Bob", 100)

	for i, peer := range peers_group1 {
		tx := *base_tx
		tx.ID = tx.ID + strconv.Itoa(i)
		peer.FloodTransaction(&tx)
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
