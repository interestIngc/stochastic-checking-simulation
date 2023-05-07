package mailbox

import (
	"log"
	"net"
	"strconv"
	"strings"
)

type Destination struct {
	To int32
	Data []byte
}

type Mailbox struct {
	id           int32                 // Process' own id
	addresses    []string              // Addresses of all processes
	writeChanMap chan Destination // Receive messages to send
	readChan     chan []byte           // Send received messages to the process

	conn *net.UDPConn
}

func NewMailbox(
	ownId int32,
	addresses []string,
	writeChanMap chan Destination,
	readChan chan []byte,
) *Mailbox {
	pC := new(Mailbox)
	pC.id = ownId
	pC.addresses = addresses
	pC.writeChanMap = writeChanMap
	pC.readChan = readChan

	address := pC.addresses[pC.id]
	log.Printf("Listening To %s.\n", address)

	addrSplit := strings.Split(address, ":")
	port, err := strconv.Atoi(addrSplit[1])
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(addrSplit[0]),
	}

	conn, err := net.ListenUDP("udp", &addr)

	if err != nil {
		log.Fatalf("P%d: Listening failed. %s\n", pC.id, err)
	}

	pC.conn = conn

	return pC
}

func (pC *Mailbox) SetUp() {
	go pC.persistentList()
	go pC.SendMessages()
}

func (pC *Mailbox) persistentList() {
	defer pC.conn.Close()

	for {
		buf := make([]byte, 1024)
		size, _, err := pC.conn.ReadFromUDP(buf)

		if err != nil {
			log.Printf("P%d: Failed when reading an incoming message. %d\n", pC.id, err)
			return
		}

		pC.readChan <- buf[:size]
	}

}

func (pC *Mailbox) SendMessages() {
	for dest := range pC.writeChanMap {
		go func(dest Destination) {
			address := make([]string, 2)
			address = strings.Split(pC.addresses[dest.To], ":")
			port, err := strconv.Atoi(address[1])
			addr := net.UDPAddr{
				Port: port,
				IP:   net.ParseIP(address[0]),
			}

			_, err = pC.conn.WriteToUDP(dest.Data, &addr)
			if err != nil {
				log.Printf("Response err %v", err)
			}
		}(dest)
	}
}
