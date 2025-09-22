package peer

import (
	"encoding/json"
	"fmt"
	"ledger/internal/account"
	"ledger/internal/util"
	"math"
	"math/rand/v2"
	"net"
	"sync"

	deadlock "github.com/sasha-s/go-deadlock"
)

type Peer struct {
	addr string
	port int

	lock sync.Mutex

	ln      net.Listener
	peers   []string
	peersMu deadlock.RWMutex
	conns   map[string]Conn
	connsMu deadlock.RWMutex

	ledger           *account.Ledger
	messageHistory   map[string]Message
	messageHistoryMu deadlock.RWMutex

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

func (p *Peer) GetLedger() *account.Ledger {
	return p.ledger
}

func (p *Peer) GetAddr() string {
	return fmtAddr(p.addr, p.port)
}

func NewPeer(addr string, port int) *Peer {
	return &Peer{
		addr:           addr,
		port:           port,
		conns:          make(map[string]Conn),
		ledger:         account.MakeLedger(),
		messageHistory: make(map[string]Message),
		done:           make(chan struct{}),
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
	go func() {
		for {
			select {
			case <-p.done:
				return
			default:
				conn, err := ln.Accept()
				if err != nil {
					// Check if err is from use of closed network connection
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
	p.messageHistoryMu.Lock()
	if _, seen := p.messageHistory[msg.Id]; seen {
		p.messageHistoryMu.Unlock()
		return nil
	}
	p.messageHistory[msg.Id] = msg
	p.messageHistoryMu.Unlock()

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
	case CmdMessageHistory:
		var history map[string]Message
		if err := json.Unmarshal(msg.Data, &history); err != nil {
			fmt.Println("Failed to unmarshal message history:", err)
			return err
		}
		// Non-Deterministic message processing order
		for _, msg := range history {
			// TODO: Don't execute DMS, implement a separate history or handleDMs
			if msg.Cmd != CmdAskForSetOfPeers {
				p.handleMessage("NO_PEER", msg)
			}
		}
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
		p.messageHistoryMu.RLock()
		resp2 := NewMessage(CmdMessageHistory, p.messageHistory)
		p.messageHistoryMu.RUnlock()
		if err := conn.enc.Encode(resp2); err != nil {
			return fmt.Errorf("failed to encode message history: %v", err)
		}
	case CmdJoin:
		var new_peer string
		if err := json.Unmarshal(msg.Data, &new_peer); err != nil {
			fmt.Println("Failed to unmarshal new peer address:", err)
			return err
		}
		if !util.Contains(p.peers, new_peer) {
			p.peers = append(p.peers, new_peer)
		}
	case CmdTransaction:
		var tx *account.Transaction
		if err := json.Unmarshal(msg.Data, &tx); err != nil {
			fmt.Println("Failed to unmarshal transaction:", err)
			return err
		}
		p.ledger.Transaction(tx)
	}
	if msg.Flood {
		p.FloodMessage(msg)
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
	p.messageHistoryMu.Lock()
	p.messageHistory[msg.Id] = msg
	p.messageHistoryMu.Unlock()
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
func (p *Peer) FloodTransaction(t *account.Transaction) {
	/* FloodMessage doesn't send message to self, so we need to update the ledger for self */
	p.ledger.Transaction(t)
	msg := NewMessage(CmdTransaction, t)
	p.FloodMessage(msg)
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
