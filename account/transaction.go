package account

import "strconv"

type Transaction struct {
	ID     string
	From   string
	To     string
	Amount int
}

func (l *Ledger) Transaction(t *Transaction) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.Accounts[t.From] -= t.Amount
	l.Accounts[t.To] += t.Amount
}

func SerializeTransaction(t *Transaction) map[string]string {
	msg := map[string]string{
		"ID":     t.ID,
		"From":   t.From,
		"To":     t.To,
		"Amount": strconv.Itoa(t.Amount),
	}
	return msg
}

func DeserializeTransaction(m map[string]string) (*Transaction, error) {
	amount, err := strconv.Atoi(m["Amount"])
	return &Transaction{
		ID:     m["ID"],
		From:   m["From"],
		To:     m["To"],
		Amount: amount,
	}, err
}
