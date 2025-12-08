package peer

import (
	blockchain_params "au_blockchain/internal"
	"au_blockchain/internal/account"
	"au_blockchain/internal/blockchain"
	"au_blockchain/internal/signature"
	"au_blockchain/internal/util"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	deadlock "github.com/sasha-s/go-deadlock"
)

type Peer struct {
	addr string
	port int

	keyPair *signature.KeyPair

	lock sync.Mutex

	ln      net.Listener
	peers   []string
	peersMu deadlock.RWMutex
	conns   map[string]Conn
	connsMu deadlock.RWMutex

	blockTree *blockchain.BlockTree

	queuedTxs   []account.SignedTransaction
	queuedTxsMu sync.RWMutex

	seenMessages   map[string]struct{}
	seenMessagesMu sync.RWMutex

	done chan struct{}
}

type Conn struct {
	conn net.Conn
	enc  *json.Encoder
	//encMu sync.Mutex
	dec *json.Decoder
}

func fmtAddr(addr string, port int) string {
	return fmt.Sprintf("%s:%d", addr, port)
}

func (p *Peer) GetPeers() []string {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()
	peers := make([]string, len(p.peers))
	copy(peers, p.peers)
	return peers
}

func (p *Peer) GetLedger() *account.Ledger {
	ledger, err := p.blockTree.GetLedgerAtHead()
	if err != nil {
		fmt.Printf("!!!!!!!!!!Error getting ledger at head: %v\n", err)
		// Should never happen
		return nil
	}
	return ledger
}

func (p *Peer) GetBlockTree() *blockchain.BlockTree {
	return p.blockTree
}

func (p *Peer) GetLuckyPeers() []string {
	threshold := 1000 // Something big so it doesn't activate yet, TODO: Check why this doesn't work
	//percentage := 0.4
	safety_margin := 3

	peers := p.GetPeers()
	self := p.GetAddr()

	candidates := peers[:0]

	for _, peer := range peers {
		if peer != self {
			candidates = append(candidates, peer)
		}
	}

	n := len(candidates)
	if n == 0 {
		return []string{}
	}
	k := n

	if n > threshold {
		k = int(math.Log(float64(n)) + float64(safety_margin))
		if k < 2 {
			k = 2
		}
		if k > n {
			k = n
		}
	}

	rand.Shuffle(n, func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	return append([]string(nil), candidates[:k]...)
}

func (p *Peer) GetAddr() string {
	return fmtAddr(p.addr, p.port)
}

func (p *Peer) GetEncodedPublicKey() string {
	return p.keyPair.Pk.Encode()
}

func NewPeer(addr string, port int, keyPair *signature.KeyPair) *Peer {
	genesisLedger := account.GetInitialGenesisLedger("genesis_pks.json")
	genesisBlock := blockchain.CreateGenesisBlock(blockchain_params.GENESIS_SEED, &genesisLedger)
	return &Peer{
		addr:           addr,
		port:           port,
		keyPair:        keyPair,
		conns:          make(map[string]Conn),
		blockTree:      blockchain.NewBlockTree(genesisBlock),
		done:           make(chan struct{}),
		seenMessages:   make(map[string]struct{}),
		seenMessagesMu: sync.RWMutex{},
	}
}

func (p *Peer) Connect(addr string, port int) error {
	fullAddr := fmtAddr(addr, port)

	err := p.Start()
	p.lock.Lock()
	defer p.lock.Unlock()
	if err != nil {
		return err
	}

	if fullAddr == p.GetAddr() {
		// Connecting to self, just return
		return nil
	}

	conn, err := net.Dial("tcp", fullAddr)
	if err != nil {
		// TODO: TEST BEING ALONE IN A NETWORK
		return nil
	}
	// Ask for set of peers
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	p.connsMu.Lock()
	p.conns[fullAddr] = Conn{conn, enc, dec}
	p.connsMu.Unlock()

	p.peersMu.Lock()
	p.peers = append(p.peers, fullAddr)
	p.peersMu.Unlock()

	go p.readLoop(fullAddr)

	// TODO: Add self.waitingForSetOfPeers = true
	request := NewMessage(CmdAskForSetOfPeers, nil)
	if err := enc.Encode(request); err != nil {
		return fmt.Errorf("failed to encode request: %v", err)
	}

	joinMessage := NewMessage(CmdJoin, p.GetAddr())
	p.FloodMessage(joinMessage)

	return nil
}

func (p *Peer) Start() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	ln, err := net.Listen("tcp", fmtAddr(p.addr, p.port))
	if err != nil {
		return err
	}
	// TODO: Test Start
	p.ln = ln
	p.peersMu.Lock()
	p.peers = []string{p.GetAddr()}
	p.peersMu.Unlock()

	go p.gambleLoop()
	go func() {
		for {
			select {
			case <-p.done:
				return
			default:
				conn, err := ln.Accept()
				if err != nil {
					// Check if err is from use of closed network connection
					fmt.Println("Error accepting connection:", err)
					return
				}
				// Make this code cleaner
				enc := json.NewEncoder(conn)
				dec := json.NewDecoder(conn)
				possible_new_peer := conn.RemoteAddr().String()

				p.connsMu.Lock()
				p.conns[possible_new_peer] = Conn{conn, enc, dec}
				p.connsMu.Unlock()
				//p.addConn(conn)
				go p.readLoop(possible_new_peer)
			}
		}
	}()
	return nil
}

func (p *Peer) gamble() {
	slot := blockchain.CurrentSlot(time.Now()) // TODO: Implement this using the SlotLength and current time
	draw := blockchain.Draw(slot, p.keyPair)
	ledger, err := p.blockTree.GetLedgerAtHead()
	if err != nil {
		// Handle error appropriately, e.g., log or return
		fmt.Println("Error getting ledger at head:", err)
		// Should not happen
		return
	}
	hash := blockchain.Hash(blockchain.LOTTERY_PREFIX, slot, p.keyPair.Pk, draw)
	hashValue := blockchain.HashValue(hash)
	value := new(big.Int).Mul(hashValue, big.NewInt(int64(ledger.GetBalance(p.GetEncodedPublicKey()))))
	if value.Cmp(big.NewInt(int64(blockchain_params.HARDNESS))) >= 0 {
		if p.GetEncodedPublicKey() == "pvJPUUUZnRRLyyem6oSEg4Ueim3Wpv8pgy/6FoG4qJmKlD8Q7QO1mQBG1ohxp0HZzlO+fMcT5HtozrLzdywk6Q==.Aw==" {
			fmt.Println("DEBUG: peer0 won the lottery for slot", slot)
			fmt.Println("DEBUG: peer0 won the lottery for slot", slot)
			fmt.Println("DEBUG: peer0 won the lottery for slot", slot)
			fmt.Println("DEBUG: peer0 won the lottery for slot", slot)
			fmt.Println("DEBUG: peer0 won the lottery for slot", slot)
		}
		// Send new block
		var txs []account.SignedTransaction
		/*
			p.queuedTxsMu.RLock()
			if len(p.queuedTxs) == 0 {
				if p.GetEncodedPublicKey() == "pvJPUUUZnRRLyyem6oSEg4Ueim3Wpv8pgy/6FoG4qJmKlD8Q7QO1mQBG1ohxp0HZzlO+fMcT5HtozrLzdywk6Q==.Aw==" {
					fmt.Println("DEBUG: peer0 has no transactions to send")
				}
				return
			}
			p.queuedTxsMu.RUnlock()
		*/
		p.queuedTxsMu.Lock()
		if len(p.queuedTxs) > blockchain_params.BLOCK_SIZE_LIMIT {
			txs = p.queuedTxs[:blockchain_params.BLOCK_SIZE_LIMIT]
			p.queuedTxs = p.queuedTxs[blockchain_params.BLOCK_SIZE_LIMIT:]
		} else {
			txs = p.queuedTxs
			p.queuedTxs = []account.SignedTransaction{}
		}
		p.queuedTxsMu.Unlock()
		newBlock := p.blockTree.CreateNewBlock(slot, draw, txs, p.GetEncodedPublicKey())
		if p.GetEncodedPublicKey() == "pvJPUUUZnRRLyyem6oSEg4Ueim3Wpv8pgy/6FoG4qJmKlD8Q7QO1mQBG1ohxp0HZzlO+fMcT5HtozrLzdywk6Q==.Aw==" {
			fmt.Println("DEBUG: peer0 created a block", newBlock)
		}
		if !p.blockTree.AddBlock(newBlock) {
			fmt.Println("Failed to add new block to block tree")
			return
		}
		blockMessage := NewMessage(CmdBlock, newBlock)
		p.FloodMessage(blockMessage)
	}
}

func (p *Peer) gambleLoop() {
	ticker := time.NewTicker(time.Second) // or SlotLength
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.gamble()
		}
	}
}

func (p *Peer) readLoop(peer string) {
	for {
		select {
		case <-p.done:
			return
		default:
		}
		p.connsMu.RLock()
		conn, exists := p.conns[peer]
		p.connsMu.RUnlock()
		if !exists {
			// Connection was already closed
			return
		}

		var msg Message
		err := conn.dec.Decode(&msg)
		if err != nil {
			// Drop connection, consider actually deleting the peer from the list
			return
		}
		err = p.handleMessage(peer, msg)
		if err != nil {
			// Same here^
			return
		}
	}
}

func (p *Peer) handleMessage(peer string, msg Message) error {
	// TODO: Check if message was already seen
	p.seenMessagesMu.RLock()
	_, seen := p.seenMessages[msg.Id]
	p.seenMessagesMu.RUnlock()
	if seen {
		return nil
	}
	p.seenMessagesMu.Lock()
	p.seenMessages[msg.Id] = struct{}{}
	p.seenMessagesMu.Unlock()

	switch msg.Cmd {
	case CmdSetOfPeers:
		// TODO: Should allow receving set of peers only once and when connecting
		var setOfPeers []string
		if err := json.Unmarshal(msg.Data, &setOfPeers); err != nil {
			fmt.Println("Failed to unmarshal set of peers:", err)
			return err
		}
		p.peersMu.Lock()
		for _, new_peer := range setOfPeers {
			if !util.Contains(p.peers, new_peer) {
				p.peers = append(p.peers, new_peer)
			}
		}
		p.peersMu.Unlock()
	case CmdAskForSetOfPeers:
		p.connsMu.RLock()
		conn, exists := p.conns[peer]
		p.connsMu.RUnlock()
		if !exists {
			return fmt.Errorf("connection to peer %s does not exist", peer)
		}
		resp := NewMessage(CmdSetOfPeers, p.GetPeers())
		if err := conn.enc.Encode(resp); err != nil {
			return fmt.Errorf("failed to encode response: %v", err)
		}
	case CmdJoin:
		var new_peer string
		if err := json.Unmarshal(msg.Data, &new_peer); err != nil {
			fmt.Println("Failed to unmarshal new peer address:", err)
			return err
		}
		p.peersMu.Lock()
		if !util.Contains(p.peers, new_peer) {
			p.peers = append(p.peers, new_peer)
		}
		p.peersMu.Unlock()

	case CmdBlock:
		var block blockchain.Block
		if err := json.Unmarshal(msg.Data, &block); err != nil {
			fmt.Println("Failed to unmarshal block:", err)
			return err
		}
		// Check if draw is valid
		// TODO: Make sure the creater of the block adds it's own CreatorPk
		if err := p.handleBlock(&block); err != nil {
			fmt.Println("Failed to handle block:", err)
			return err
		}

	}
	if msg.Flood {
		p.FloodMessage(msg)
	}
	return nil
}

func (p *Peer) handleBlock(block *blockchain.Block) error {
	if err := p.blockTree.CheckBlock(block); err != nil {
		return fmt.Errorf("block check failed: %v", err)
	}

	if !p.blockTree.AddBlock(block) {
		return fmt.Errorf("failed to add block to block tree")
	}

	return nil
}

func (p *Peer) ensureConnection(peer string) error {
	p.connsMu.Lock()
	defer p.connsMu.Unlock()
	if _, exists := p.conns[peer]; exists {
		return nil
	}

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	p.conns[peer] = Conn{conn, enc, dec}

	go p.readLoop(peer)

	return nil
}

func (p *Peer) FloodMessage(msg Message) {
	msg.Flood = true
	peers := p.GetLuckyPeers()
	for _, peer := range peers {
		if peer == p.GetAddr() {
			continue
		}
		if err := p.ensureConnection(peer); err != nil {
			//fmt.Println("Failed to ensure connection:", err)
			// TODO: Consider deleting the peer from the list when
			// reimplementing the Disconnect better
			continue
		}
		p.connsMu.RLock()
		conn, exists := p.conns[peer]
		p.connsMu.RUnlock()
		if !exists {
			continue
		}
		if err := conn.enc.Encode(msg); err != nil {
			continue
		}
	}
}

func (p *Peer) Disconnect() {
	close(p.done)

	p.lock.Lock()
	defer p.lock.Unlock()

	if p.ln != nil {
		// This will cause ln.Accept() to return error
		p.ln.Close()
	}

	p.connsMu.Lock()
	for peer, conn := range p.conns {
		conn.conn.Close()
		delete(p.conns, peer)
	}
	p.connsMu.Unlock()
	// TODO: TEST Disconnect
}

func (p *Peer) CreateTransaction(to string, amount int) *account.SignedTransaction {
	tx := account.NewSignedTransaction(
		p.GetEncodedPublicKey(),
		to,
		amount,
		p.keyPair.Sk,
	)
	return tx
}

func (p *Peer) SendBalance(to string, amount int) {
	tx := p.CreateTransaction(to, amount)
	p.queuedTxsMu.Lock()
	defer p.queuedTxsMu.Unlock()
	p.queuedTxs = append(p.queuedTxs, *tx)
}
