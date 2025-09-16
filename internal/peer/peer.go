package peer

import (
	"encoding/json"
	"fmt"
	"ledger/internal/account"
	"ledger/internal/util"
	"math/rand/v2"
	"net"
	"sync"
)

type Conn struct {
	conn net.Conn
	enc  *json.Encoder
	//encMu sync.Mutex
	dec *json.Decoder
}

type Peer struct {
	addr string
	port int

	lock sync.Mutex

	ln      net.Listener
	peers   []string
	peersMu sync.RWMutex
	conns   map[string]Conn
	connsMu sync.RWMutex

	ledger           *account.Ledger
	messageHistory   map[string]Message
	messageHistoryMu sync.Mutex

	done chan struct{}
}

func (p *Peer) GetPeers() []string {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()
	peers := make([]string, len(p.peers))
	copy(peers, p.peers)
	return peers
}

func (p *Peer) GetLuckyPeers() []string {
	threshold := 10
	percentage := 1.0
	p.peersMu.Lock()
	defer p.peersMu.Unlock()
	// Deep copy p.peers before returning
	peers := make([]string, len(p.peers))
	copy(peers, p.peers)
	num_peers := len(peers)
	if num_peers > threshold {
		num_peers = int(float64(num_peers) * percentage)
	}
	perm := rand.Perm(num_peers)
	lucky_peers := make([]string, num_peers)
	for i := 0; i < num_peers; i++ {
		lucky_peers[i] = peers[perm[i]]
	}
	peers = lucky_peers
	return peers
}

func fmtAddr(addr string, port int) string {
	return fmt.Sprintf("%s:%d", addr, port)
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

func (p *Peer) GetLedger() *account.Ledger {
	return p.ledger
}

func (p *Peer) Connect(addr string, port int) error {
	addr = fmtAddr(addr, port)

	err := p.Start()
	p.lock.Lock()
	defer p.lock.Unlock()
	if err != nil {
		return err
	}

	if addr == p.Addr() {
		// Connecting to self, just return
		return nil
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		// It's ok to fail, it may mean connecting to self
		// TODO: TEST BEING ALONE IN A NETWORK
		return nil
	}
	// Ask for set of peers
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	p.connsMu.Lock()
	p.conns[fmtAddr(addr, port)] = Conn{conn, enc, dec}
	p.connsMu.Unlock()

	request := NewMessage(CmdAskForSetOfPeers, nil)
	enc.Encode(request)

	var reply Message
	err = dec.Decode(&reply)
	if err != nil {
		return err
	}

	var setOfPeers []string
	//reply.Cmd == CmdSendSetOfPeers
	// TODO: Also check CMD (Because another message can arrive before, there should be a channel in the loop that check this)
	if err := json.Unmarshal(reply.Data, &setOfPeers); err != nil {
		fmt.Println("Invalid response when requested set of peers")
		//panic(-1)
	}
	p.peers = append(setOfPeers, p.Addr())
	// TODO: TEST set of peers

	joinMessage := NewMessage(CmdJoin, p.Addr())
	p.FloodMessage(joinMessage)

	return nil
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

func (p *Peer) Addr() string {
	return fmtAddr(p.addr, p.port)
}

func (p *Peer) FloodMessage(msg Message) {
	msg.Flood = true
	p.messageHistoryMu.Lock()
	p.messageHistory[msg.Id] = msg
	p.messageHistoryMu.Unlock()
	peers := p.GetLuckyPeers() // Get all peers instead of random
	for _, peer := range peers {
		if peer == p.Addr() {
			continue
		}
		p.ensureConnection(peer)
		p.connsMu.Lock()
		p.conns[peer].enc.Encode(msg)
		p.connsMu.Unlock()
	}
}

func (p *Peer) ensureConnection(peer string) error {
	// Already locked from caller Connect()
	//p.lock.Lock()
	//defer p.lock.Unlock()

	p.connsMu.Lock()
	defer p.connsMu.Unlock()
	if _, exists := p.conns[peer]; exists {
		return nil
	}

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		fmt.Println("Cannot connect to peer: ", peer)
		panic(-1)
	}
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	p.conns[peer] = Conn{conn, enc, dec}

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
	p.peers = []string{p.Addr()}
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
					//panic(err)
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
		p.connsMu.Lock()
		conn, ok := p.conns[peer]
		p.connsMu.Unlock()
		if !ok {
			// Connection was closed
			return
		}

		var msg Message
		err := conn.dec.Decode(&msg)
		if err != nil {
			fmt.Println("Failed to decode message on message loop:", err)
			p.connsMu.Lock()
			if c, exists := p.conns[peer]; exists {
				c.conn.Close()
				delete(p.conns, peer)
			}
			p.connsMu.Unlock()
			return
		}
		err = p.handleMessage(peer, msg)
		if err != nil {
			fmt.Println("Failed to handle message on message loop")
			//delete(p.conns, peer)
			//return
			panic(-1) // Should not happen, no currupted messages allowed for now
		}
	}
}

func (p *Peer) FloodTransaction(t *account.Transaction) {
	/* Currently FloodMessage doesn't send message to self, so we need to update the ledger for self */
	// TODO: Check if t.ID was already executed
	p.ledger.Transaction(t)
	msg := NewMessage(CmdTransaction, t)
	p.FloodMessage(msg)
}

func (p *Peer) handleMessage(peer string, msg Message) error {
	p.messageHistoryMu.Lock()
	if _, seen := p.messageHistory[msg.Id]; seen {
		p.messageHistoryMu.Unlock()
		return nil
	}
	p.messageHistory[msg.Id] = msg
	p.messageHistoryMu.Unlock()

	conn := p.conns[peer]
	switch msg.Cmd {
	case CmdAskForSetOfPeers:
		resp := NewMessage(CmdSendSetOfPeers, p.peers)
		if err := conn.enc.Encode(resp); err != nil {
			return fmt.Errorf("failed to encode response: %v", err)
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
