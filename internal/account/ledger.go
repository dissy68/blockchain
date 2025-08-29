package account

import (
	"sync"
)

type Ledger struct {
	Accounts map[string]int
	lock     sync.Mutex
}

func MakeLedger() *Ledger {
	ledger := new(Ledger)
	ledger.Accounts = make(map[string]int)
	return ledger
}

func VerifyLedgerConsistency(ledgers []*Ledger) bool {
	ledger0 := ledgers[0]
	for i := 1; i < len(ledgers); i++ {
		ledger := ledgers[i]
		if len(ledger0.Accounts) != len(ledger.Accounts) {
			return false
		}
		for acc, bal := range ledger0.Accounts {
			if ledger.Accounts[acc] != bal {
				return false
			}
		}
	}
	return true
}
