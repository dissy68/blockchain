package peer

import (
	"encoding/json"
	"fmt"
	"ledger/account"
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

	//Done chan struct{} // Not used yet, do a better less abrupt disconnect
}

type Message struct {
	Cmd   string `json:"cmd"`
	Flood bool   `json:"flood_message,omitempty"`
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
		fmt.Println("DEBUG: New network", p.peers)
		return nil
	}
	// Ask for set of peers
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	p.conns[fmtAddr(addr, port)] = Conn{conn, enc, dec}

	enc.Encode(map[string]string{"cmd": "ask_for_set_of_peers"})

	var reply map[string][]string
	err = dec.Decode(&reply)
	if err != nil {
		return err
	}

	if peers, ok := reply["set_of_peers"]; ok {
		p.peers = peers
	} else {
		fmt.Println("Invalid response when requested set of peers")
		panic(-1)
	}
	p.peers = append(p.peers, p.Addr())
	fmt.Println("Set of peers: ", p.peers)

	joinMessage := map[string]string{"cmd": "join", "peer": p.Addr()}

	p.FloodMessage(joinMessage)
	return nil
}

func (p *Peer) Disconnect() {
	p.ln.Close()
	for _, conn := range p.conns {
		conn.conn.Close()
	}
}

func (p *Peer) Addr() string {
	return fmtAddr(p.addr, p.port)
}

func (p *Peer) FloodMessage(msg map[string]string) {
	msg["flood_message"] = "true"
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
	fmt.Println("I am listening on ", fmtAddr(p.addr, p.port))
	p.ln = ln
	p.peers = []string{p.Addr()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				// Probably ln.Close() called
				// return
				panic(err)
			}
			// Make this code cleaner
			enc := json.NewEncoder(conn)
			dec := json.NewDecoder(conn)
			possible_new_peer := conn.RemoteAddr().String()
			p.conns[possible_new_peer] = Conn{conn, enc, dec}
			//p.addConn(conn)
			go p.readLoop(possible_new_peer)
		}
	}()
	return nil
}

func (p *Peer) readLoop(peer string) {
	for {
		var msg map[string]string
		conn := p.conns[peer]
		err := conn.dec.Decode(&msg)
		if err != nil {
			fmt.Println("Failed to decode message on message loop")
			//delete(p.conns, peer)
			//return
			panic(-1)
		}
		err = p.handleMessage(peer, msg)
		if err != nil {
			fmt.Println("Failed to handle message on message loop")
			//delete(p.conns, peer)
			//return
			panic(-1)
		}
	}
}

func (p *Peer) FloodTransaction(t *account.Transaction) {
	/* Currently FloodMessage doesn't send message to self, so we need to update the ledger for self */
	p.ledger.Transaction(t)
	msg := account.SerializeTransaction(t)
	msg["cmd"] = "transaction"
	p.FloodMessage(msg)
}

func (p *Peer) handleMessage(peer string, msg map[string]string) error {
	conn := p.conns[peer]
	switch msg["cmd"] {
	case "ask_for_set_of_peers":
		resp := map[string][]string{"set_of_peers": p.peers}
		conn.enc.Encode(resp)
	case "join":
		new_peer := msg["peer"]
		p.peers = append(p.peers, new_peer)
	case "transaction":
		tx, err := account.DeserializeTransaction(msg)
		if err != nil {
			fmt.Println("Failed ot deserialize transaction")
			return err
		}
		p.ledger.Transaction(tx)
	}
	if msg["flood_message"] == "true" {
		// Make sure to check if already seen by others or already sent
		// Also almost all messages should be flooded, so check if forged messages
		//p.FloodMessage(msg)
	}
	return nil
}
