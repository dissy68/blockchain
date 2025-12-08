package account

import (
	"fmt"
	"sync"
)

type Ledger struct {
	Accounts map[string]int
	lock     sync.RWMutex
}

func (l *Ledger) GetBalance(account string) int {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.Accounts[account]
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

func (l *Ledger) CheckTransactions(txs []SignedTransaction) bool {
	l.lock.RLock()
	defer l.lock.RUnlock()
	tempLedger := MakeLedger()
	for k, v := range l.Accounts {
		tempLedger.Accounts[k] = v
	}
	for _, tx := range txs {
		if err := tempLedger.ExecuteSignedTransaction(&tx); err != nil {
			return false
		}
	}
	return true
}

func (l *Ledger) Display() {
	l.lock.RLock()
	defer l.lock.RUnlock()
	for acc, bal := range l.Accounts {
		fmt.Printf("Account: %s, Balance: %d\n", acc, bal)
	}
	fmt.Println()
	fmt.Println()
}

func (l *Ledger) Copy() *Ledger {
	newLedger := MakeLedger()
	for account, balance := range l.Accounts {
		newLedger.Accounts[account] = balance
	}
	return newLedger
}
