package mailbox

import (
	"log"
	"math"
	"net"
	"strconv"
	"strings"
)

const BufferSize = 1024

var ReadBufferSize = int(math.Pow(2, 20))

type Packet struct {
	To   int32
	Data []byte
}

type Mailbox struct {
	id           int32          // Process own id
	udpAddresses []*net.UDPAddr // UDP addresses of all processes
	writeChannel chan Packet    // Receive messages to send
	readChannel  chan []byte    // Send messages to other processes

	conn *net.UDPConn
}

func NewMailbox(
	ownId int32,
	addresses []string,
	writeChannel chan Packet,
	readChannel chan []byte,
) *Mailbox {
	m := new(Mailbox)
	m.id = ownId
	m.writeChannel = writeChannel
	m.readChannel = readChannel

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

	return m
}

func (m *Mailbox) SetUp() {
	go m.listenForMessages()
	go m.sendMessages()
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
		go func(packet Packet) {
			_, err := m.conn.WriteToUDP(packet.Data, m.udpAddresses[packet.To])

			if err != nil {
				log.Printf("Could not write data to udp: %e", err)
			}
		}(packet)
	}
}
