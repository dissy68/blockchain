package account

type Transaction struct {
	ID     string
	From   string
	To     string
	Amount int
}

func NewTransaction(id, from, to string, amount int) *Transaction {
	return &Transaction{
		ID:     id,
		From:   from,
		To:     to,
		Amount: amount,
	}
}

func (l *Ledger) Transaction(t *Transaction) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.Accounts[t.From] -= t.Amount
	l.Accounts[t.To] += t.Amount
}
