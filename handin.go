package main

import (
	"fmt"
	"ledger/internal/account"
	"ledger/internal/peer"
	"math/rand"
	"strconv"
	"time"
)

const BASE_PORT = 10000

func main() {
	rand.Seed(time.Now().UnixNano())

	numPeers := 10
	txPerPeer := 10

	peers := peer.CreateTestNetwork(numPeers, BASE_PORT)
	defer peer.CleanupPeers(peers)

	computedLedger := account.MakeLedger()

	for i, p := range peers {
		for j := range txPerPeer {
			tx := account.NewTransaction(
				"tx"+strconv.Itoa(i),
				"account"+strconv.Itoa(j%5),
				"account"+strconv.Itoa((j+1)%5),
				rand.Intn(100),
			)
			computedLedger.Transaction(tx)
			p.FloodTransaction(tx)
		}
	}

	time.Sleep(1 * time.Second)

	consistent := verifyLedgersConsistent(peers, computedLedger)
	if !consistent {
		fmt.Println("Error: Ledgers are inconsistent or do not match computed ledger")
	} else {
		fmt.Println("Success: All ledgers are consistent and match computed ledger")
	}
}

func verifyLedgersConsistent(peers []*peer.Peer, computed *account.Ledger) bool {
	for i := 1; i < len(peers); i++ {
		if !compareLedgers(peers[0].GetLedger(), peers[i].GetLedger()) {
			return false
		}
	}

	return compareLedgers(peers[0].GetLedger(), computed)
}

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
