package peer

import (
	"encoding/json"
	"fmt"
	"ledger/internal/account"
	"net"
	"sync"
)

type Conn struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

type Peer struct {
	addr string
	port int

	lock  sync.Mutex
	ln    net.Listener
	peers []string
	conns map[string]Conn

	ledger *account.Ledger

	done chan struct{}

	//Done chan struct{} // Not used yet, do a better less abrupt disconnect
}

func (p *Peer) GetPeers() []string {
	p.lock.Lock()
	defer p.lock.Unlock()
	// Deep copy p.peers before returning
	peers := make([]string, len(p.peers))
	copy(peers, p.peers)
	return peers
}

func fmtAddr(addr string, port int) string {
	return fmt.Sprintf("%s:%d", addr, port)
}

func NewPeer(addr string, port int) *Peer {
	return &Peer{
		addr:   addr,
		port:   port,
		conns:  make(map[string]Conn),
		ledger: account.MakeLedger(),
		done:   make(chan struct{}),
	}
}

func (p *Peer) GetLedger() *account.Ledger {
	return p.ledger
}

func (p *Peer) Connect(addr string, port int) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	conn, err := net.Dial("tcp", fmtAddr(addr, port))
	if err != nil {
		// New network
		// TODO: TEST BEING ALONE IN A NETWORK
		return nil
	}
	// Ask for set of peers
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	p.conns[fmtAddr(addr, port)] = Conn{conn, enc, dec}

	request := Message{Cmd: "ask_for_set_of_peers", Data: nil}
	enc.Encode(request)

	var reply Message
	err = dec.Decode(&reply)
	if err != nil {
		return err
	}

	var setOfPeers []string
	// TODO: Also check CMD (Because another message can arrive before, there should be a channel in the loop that check this)
	if err := json.Unmarshal(reply.Data, &setOfPeers); err != nil {
		fmt.Println("Invalid response when requested set of peers")
		panic(-1)
	}
	p.peers = append(setOfPeers, p.Addr())
	// TODO: TEST set of peers

	joinMessage := NewMessage("join", p.Addr())

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

	for peer, conn := range p.conns {
		conn.conn.Close()
		delete(p.conns, peer)
	}
	// TODO: TEST Disconnect
}

func (p *Peer) Addr() string {
	return fmtAddr(p.addr, p.port)
}

func (p *Peer) FloodMessage(msg Message) {
	msg.Flood = true
	for _, peer := range p.peers {
		if peer == p.Addr() {
			continue
		}
		p.ensureConnection(peer)
		p.conns[peer].enc.Encode(msg)
	}
}

func (p *Peer) ensureConnection(peer string) error {
	// Already locked from caller Connect()
	//p.lock.Lock()
	//defer p.lock.Unlock()

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

	// go p.readLoop(peer)

	return nil
}

func (p *Peer) Start() error {
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
					select {
					case <-p.done:
						return // We shouldn't be in here anyways, due to the select above
					default:
						panic(err)
					}
				}
				// Make this code cleaner
				enc := json.NewEncoder(conn)
				dec := json.NewDecoder(conn)
				possible_new_peer := conn.RemoteAddr().String()

				p.lock.Lock()
				p.conns[possible_new_peer] = Conn{conn, enc, dec}
				p.lock.Unlock()
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
		p.lock.Lock()
		conn, ok := p.conns[peer]
		p.lock.Unlock()
		if !ok {
			// Connection was closed
			return
		}

		var msg Message
		err := conn.dec.Decode(&msg)
		if err != nil {
			select {
			case <-p.done:
				return
			default:
				fmt.Println("Failed to decode message on message loop:", err)
				p.lock.Lock()
				if c, exists := p.conns[peer]; exists {
					c.conn.Close()
					delete(p.conns, peer)
				}
				p.lock.Unlock()
				return
			}
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
	msg := NewMessage("transaction", t)
	p.FloodMessage(msg)
}

func (p *Peer) handleMessage(peer string, msg Message) error {
	conn := p.conns[peer]
	switch msg.Cmd {
	case "ask_for_set_of_peers":
		resp := NewMessage("set_of_peers", p.peers)
		if err := conn.enc.Encode(resp); err != nil {
			return fmt.Errorf("failed to encode response: %v", err)
		}
	case "join":
		var new_peer string
		if err := json.Unmarshal(msg.Data, &new_peer); err != nil {
			fmt.Println("Failed to unmarshal new peer address:", err)
			return err
		}
		p.peers = append(p.peers, new_peer)
	case "transaction":
		var tx *account.Transaction
		if err := json.Unmarshal(msg.Data, &tx); err != nil {
			fmt.Println("Failed to unmarshal transaction:", err)
			return err
		}
		p.ledger.Transaction(tx)
	}
	if msg.Flood {
		// Make sure to check if already seen by others or already sent
		// Also almost all messages should be flooded, so check if forged messages
		//p.FloodMessage(msg)
	}
	return nil
}
