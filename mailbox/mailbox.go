package mailbox

import (
	"log"
	"net"
)

type Mailbox struct {
	id           int64           // Process' own id
	addresses    []string
	writeChanMap map[int64]chan []byte // Receive messages to send
	readChan     chan []byte           // Send received messages to the process

	ls net.Listener

	writeConns map[int64]net.Conn // Connections to write messages
}

func NewMailbox(
	ownId int64,
	addresses []string,
	writeChanMap map[int64]chan []byte,
	readChan chan []byte,
) *Mailbox {
	pC := new(Mailbox)
	pC.id = ownId
	pC.addresses = addresses
	pC.writeChanMap = writeChanMap
	pC.readChan = readChan
	pC.writeConns = make(map[int64]net.Conn)

	address := pC.addresses[pC.id]
	log.Printf("Listening To %s.\n", address)
	ls, err := net.Listen("tcp", address)

	if err != nil {
		log.Fatalf("%d: Listening failed. %d\n", pC.id, err)
	}

	pC.ls = ls

	return pC
}

func (pC *Mailbox) SetUp() {
	go pC.waitForReadConnections(pC.ls)
	for to, inChan := range pC.writeChanMap {
		go pC.SendMessages(to, inChan)
	}
}

func (pC *Mailbox) waitForReadConnections(ls net.Listener) {
	for {
		conn, err := ls.Accept()

		if err != nil {
			log.Printf("%d: Connection failed. %d\n", pC.id, err)
			conn.Close()
		} else {
			go pC.handleConnection(conn)
		}
	}
}

func (pC *Mailbox) handleConnection(pConn net.Conn) {
	from := pConn.RemoteAddr().String()
	log.Printf("%d: Connection established with %s.\n", pC.id, from)
	
	//data, e := pC.readFromConnection(pConn)
	//if e != nil {
	//	return
	//}
	//
	//msg := &messages.ConnMessage{}
	//e = proto.Unmarshal(data, msg)
	//if e != nil {
	//	fmt.Printf("Could not unmarshal message: %v", data)
	//	return
	//}

	pC.readMessages(pConn)
}

func (pC *Mailbox) readMessages(pConn net.Conn) {
	for {
		buf, err := pC.readFromConnection(pConn)

		if err != nil {
			return
		}

		pC.readChan <- buf
	}
}

//func (pC *Mailbox) ConnectToPeers() {
//	for i := 0; i < len(pC.addresses); i++ {
//		to := int64(i)
//		_, connected := pC.writeConns[to]
//		if !connected {
//			pC.Connect(to)
//		}
//	}
//}

func (pC *Mailbox) Connect(to int64) net.Conn {
	conn := pC.doConnect(to)
	pC.writeConns[to] = conn
	return conn
}

func (pC *Mailbox) doConnect(to int64) net.Conn {
	log.Printf("%d: Connecting to %d", pC.id, to)
	addr := pC.addresses[to]
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		log.Printf("%s: Failed to establish a connection to %s, retrying. %e\n", pC.addresses[pC.id], addr, err)
		return pC.doConnect(to)
	}

	//msg := &messages.ConnMessage{
	//	From: pC.id,
	//	Stub: 1,
	//}
	//data, e := proto.Marshal(msg)
	//if e != nil {
	//	log.Printf("Error while marshaling happened: %e", e)
	//	return nil
	//}
	//pC.writeIntoConnection(conn, data)

	return conn
}

func (pC *Mailbox) SendMessages(to int64, inChan chan []byte) {
	conn, connected := pC.writeConns[to]
	for data := range inChan {
		if !connected {
			conn = pC.Connect(to)
			connected = true
		}
		//log.Printf("Sending message to %d\n", to)
		for {
			err := pC.writeIntoConnection(conn, data)
			if err == nil {
				break
			}
			log.Printf("Sending to %d, could not write data, reconnecting\n", to)
			conn = pC.Connect(to)
		}
	}
}

func (pC *Mailbox) writeIntoConnection(conn net.Conn, data []byte) error {
	// Adding message size on first byte
	m_ := make([]byte, len(data)+1)
	m_[0] = byte(len(data))

	for i := 0; i < len(data); i++ {
		m_[i+1] = data[i]
	}

	_, err := conn.Write(m_)

	return err
}

func (pC *Mailbox) readFromConnection(conn net.Conn) ([]byte, error) {
	// Reading size byte first
	buf := make([]byte, 1)
	_, err := conn.Read(buf)

	buf = make([]byte, int64(buf[0]))
	_, err = conn.Read(buf)

	if err != nil {
		log.Printf("%d: Failed when reading an incoming message. %d\n", pC.id, err)
		conn.Close()
		return buf, err
	}
	
	return buf, nil
}
