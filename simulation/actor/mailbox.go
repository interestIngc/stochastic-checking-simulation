package actor

import (
	"log"
	"math"
	"net"
	"stochastic-checking-simulation/context"
	"strconv"
	"strings"
	"time"
)

//const BufferSize = 1024
const BufferSize = 65536

var ReadBufferSize = int(math.Pow(2, 20))

// Mailbox allows an actor to read and send messages from or to other actors in the system.
type Mailbox struct {
	id           int32               // Process own id
	udpAddresses []*net.UDPAddr      // UDP addresses of all processes
	writeChannel chan context.Packet // Receive messages to send
	readChannel  chan []byte         // Send messages to other processes

	conn *net.UDPConn

	messageDelay int
}

func newMailbox(
	ownId int32,
	addresses []string,
	writeChannel chan context.Packet,
	readChannel chan []byte,
	messageDelay int,
) *Mailbox {
	m := new(Mailbox)
	m.id = ownId
	m.writeChannel = writeChannel
	m.readChannel = readChannel
	m.messageDelay = messageDelay

	m.udpAddresses = make([]*net.UDPAddr, len(addresses))
	for i, currAddress := range addresses {
		hostAndPort := strings.Split(currAddress, ":")
		port, err := strconv.Atoi(hostAndPort[1])
		if err != nil {
			log.Fatalf("Port %s is not an integer value\n", hostAndPort[1])
		}
		m.udpAddresses[i] = &net.UDPAddr{
			Port: port,
			IP:   net.ParseIP(hostAndPort[0]),
		}
	}

	log.Printf("Listening To %s.\n", addresses[m.id])

	conn, err := net.ListenUDP("udp", m.udpAddresses[m.id])
	if err != nil {
		log.Fatalf("P%d: Listening failed: %e\n", m.id, err)
	}

	err = conn.SetReadBuffer(ReadBufferSize)
	if err != nil {
		log.Fatalf("P%d: Could not set read buffer size to %d", m.id, ReadBufferSize)
	}

	m.conn = conn

	go m.listenForMessages()
	go m.sendMessages()

	return m
}

func (m *Mailbox) listenForMessages() {
	defer m.conn.Close()

	for {
		buf := make([]byte, BufferSize)
		size, _, err := m.conn.ReadFromUDP(buf)

		if err != nil {
			log.Printf("P%d: Failed when reading an incoming message: %d\n", m.id, err)
			return
		}

		m.readChannel <- buf[:size]
	}
}

func (m *Mailbox) sendMessages() {
	for packet := range m.writeChannel {
		_, err := m.conn.WriteToUDP(packet.Data, m.udpAddresses[packet.To])

		if err != nil {
			log.Printf("Could not write data to udp: %e", err)
		}

		time.Sleep(time.Duration(m.messageDelay))
	}
}
