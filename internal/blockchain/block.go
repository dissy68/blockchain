package blockchain

import (
	blockchain_params "au_blockchain/internal"
	"au_blockchain/internal/account"
	"au_blockchain/internal/signature"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
)

type Block struct {
	PreviousHash string
	Hash         string
	Slot         int
	CreatorPk    string
	Draw         *big.Int
	Transactions []account.SignedTransaction
}

func (b *Block) CalculateHash() string {
	h := sha256.New()

	h.Write([]byte(b.PreviousHash))

	var slotBytes [8]byte
	binary.BigEndian.PutUint64(slotBytes[:], uint64(b.Slot))
	h.Write(slotBytes[:])

	h.Write([]byte(b.Draw.String()))

	for _, tx := range b.Transactions {
		h.Write(account.Serialize(&tx))
	}

	return hex.EncodeToString(h.Sum(nil))
}

/*
GenesisBlock is just a Block with:
PreviousHash == ""
Hash == seed
Slot == 0
Transactions == [("GENESIS" to initial accounts), ...]
*/

func (b *Block) IsGenesis() bool {
	return b.PreviousHash == ""
}

type BlockNode struct {
	Block    *Block
	Parent   *BlockNode
	Children []*BlockNode
}

/*
GenesisBlockNode is just a BlockNode with:
Parent == nil
*/

func (bn *BlockNode) IsGenesis() bool {
	return bn.Parent == nil
}

func (bn *BlockNode) IsLeaf() bool {
	return len(bn.Children) == 0
}

type BlockTree struct {
	mu    sync.RWMutex
	nodes map[string]*BlockNode

	currentHead   string // The hash of the current head block
	currentLedger *account.Ledger
	//heads        []*BlockNode
}

// TODO: Optimize ledger retrieval with caching
func (bt *BlockTree) GetLedgerAtBlock(blockHash string) (*account.Ledger, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	node, exists := bt.nodes[blockHash]
	if !exists {
		return nil, fmt.Errorf("block hash not found in block tree")
	}
	ledger := account.MakeLedger()
	var blocks []*Block
	for n := node; n != nil; n = n.Parent {
		if n.Block.Hash == bt.currentHead {
			ledger = bt.currentLedger.Copy() // Deep copy cached ledger
			break
		}
		blocks = append(blocks, n.Block)
	}
	// Apply blocks in reverse order
	for i := len(blocks) - 1; i >= 0; i-- {
		block := blocks[i]
		for _, tx := range block.Transactions {
			if block.IsGenesis() {
				ledger.ExecuteGenesisTransactions(&tx)
			} else if err := ledger.ExecuteSignedTransaction(&tx); err != nil {
				return nil, fmt.Errorf("failed to execute transaction %s: %v", tx.ID, err)
			}
		}
		// Add money to the creator of the block
		if !block.IsGenesis() && block.CreatorPk != "" {
			creatorBalance := ledger.GetBalance(block.CreatorPk)
			newBalance := creatorBalance + blockchain_params.BLOCK_REWARD
			ledger.Accounts[block.CreatorPk] = newBalance
		}
	}
	return ledger, nil
}

func (bt *BlockTree) GetLedgerAtHead() (*account.Ledger, error) {
	head := bt.GetHead()
	if head.Block.Hash == bt.currentHead {
		return bt.currentLedger, nil
	}
	ledger, err := bt.GetLedgerAtBlock(head.Block.Hash)
	if err != nil {
		return nil, err
	}
	bt.currentHead = head.Block.Hash
	bt.currentLedger = ledger
	return ledger, nil
}

func CreateGenesisBlock(seed string, initialLedger *account.Ledger) *Block {
	genesisKeyPair := signature.DefaultKeyGen()
	txs := make([]account.SignedTransaction, 0, len(initialLedger.Accounts))
	for acc, bal := range initialLedger.Accounts {
		txs = append(txs, *account.NewSignedTransaction("GENESIS", acc, bal, genesisKeyPair.Sk))
	}
	genesisBlock := &Block{
		PreviousHash: "",
		Hash:         seed,
		Slot:         0,
		Transactions: txs,
	}
	return genesisBlock
}

func NewBlockTree(genesisBlock *Block) *BlockTree {
	genesisNode := &BlockNode{
		Block:    genesisBlock,
		Parent:   nil,
		Children: []*BlockNode{},
	}
	return &BlockTree{
		mu:    sync.RWMutex{},
		nodes: map[string]*BlockNode{genesisBlock.Hash: genesisNode},
		//heads:        []*BlockNode{genesisNode},
	}
}

func (bt *BlockTree) CheckBlock(block *Block) error {
	// TODO: Check the lock scope
	/*
		bt.mu.RLock()
		defer bt.mu.RUnlock()
	*/
	ledger, err := bt.GetLedgerAtBlock(block.PreviousHash)
	if err != nil {
		return err
	}
	creatorPk, err := signature.DecodePk(block.CreatorPk)
	if err != nil {
		return fmt.Errorf("failed to decode creator public key: %v", err)
	}
	if !VerifyDraw(block.Slot, creatorPk, block.Draw) {
		return fmt.Errorf("invalid draw signature")
	}
	hash := Hash(LOTTERY_PREFIX, block.Slot, creatorPk, block.Draw)
	hashValue := HashValue(hash)
	value := new(big.Int).Mul(hashValue, big.NewInt(int64(ledger.GetBalance(block.CreatorPk))))
	if value.Cmp(big.NewInt(int64(blockchain_params.HARDNESS))) < 0 {
		// Invalid draw, ignore block
		return fmt.Errorf("invalid draw")
	}
	// TODO: Check if slot of the current block is after the slot of it's head
	if len(block.Transactions) > blockchain_params.BLOCK_SIZE_LIMIT {
		return fmt.Errorf("block size limit exceeded")
	}
	for _, tx := range block.Transactions {
		if !tx.Verify() {
			return fmt.Errorf("invalid transaction signature in block")
		}
	}
	if !ledger.CheckTransactions(block.Transactions) {
		return fmt.Errorf("invalid transactions in block")
	}
	return nil
}

func (bt *BlockTree) AddBlock(block *Block) bool {
	if err := bt.CheckBlock(block); err != nil {
		fmt.Println("Block rejected:", err)
		return false
	}

	bt.mu.Lock()
	defer bt.mu.Unlock()
	parentNode, exists := bt.nodes[block.PreviousHash]
	if !exists {
		return false
	}

	newNode := &BlockNode{
		Block:    block,
		Parent:   parentNode,
		Children: []*BlockNode{},
	}
	parentNode.Children = append(parentNode.Children, newNode)
	bt.nodes[block.Hash] = newNode
	return true
}

func (bt *BlockTree) GetHead() *BlockNode {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	var head *BlockNode
	maxDepth := -1
	var dfs func(node *BlockNode, depth int)
	dfs = func(node *BlockNode, depth int) {
		if node.IsLeaf() {
			if depth > maxDepth {
				maxDepth = depth
				head = node
			}
		} else {
			for _, child := range node.Children {
				dfs(child, depth+1)
			}
		}
	}
	// Start DFS from the genesis block node
	for _, node := range bt.nodes {
		if node.IsGenesis() {
			dfs(node, 0)
			break
		}
	}
	return head
}

func (bt *BlockTree) CreateNewBlock(slot int, draw *big.Int, transactions []account.SignedTransaction, creatorPk string) *Block {
	//bt.mu.RLock()
	//defer bt.mu.RUnlock()
	head := bt.GetHead()
	newBlock := &Block{
		PreviousHash: head.Block.Hash,
		Slot:         slot,
		Draw:         draw,
		CreatorPk:    creatorPk,
		Transactions: transactions,
	}
	newBlock.Hash = newBlock.CalculateHash()
	return newBlock
}

func (bt *BlockTree) Display() {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	var displayNode func(node *BlockNode, level int)
	displayNode = func(node *BlockNode, level int) {
		prefix := ""
		for i := 0; i < level; i++ {
			prefix += "  "
		}
		fmt.Printf("%s- Block Hash: %s, Previous Hash: %s, Slot: %d, Transactions: %d\n",
			prefix, node.Block.Hash, node.Block.PreviousHash, node.Block.Slot, len(node.Block.Transactions))
		for _, child := range node.Children {
			displayNode(child, level+1)
		}
	}
	// Start displaying from the genesis block node
	for _, node := range bt.nodes {
		if node.IsGenesis() {
			displayNode(node, 0)
			break
		}
	}
}
