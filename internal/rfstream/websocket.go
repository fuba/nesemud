// SPDX-License-Identifier: MIT

package rfstream

import (
	"encoding/binary"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

const (
	webSocketBundleMagic           = 0x4e434231 // "NCB1"
	webSocketBundleVersion         = 1
	webSocketBundleHeaderSize      = 8
	webSocketMaxBundleBytes        = 256 << 10
	webSocketMaxBundlePackets      = 1024
	webSocketClientQueue           = 32
	webSocketMaxClients            = 2
	webSocketMaxClientMessageBytes = 125
)

type webSocketRelayStats struct {
	Clients     int
	Bundles     uint64
	Packets     uint64
	Bytes       uint64
	Disconnects uint64
}

type webSocketRelayClient struct {
	queue          chan []byte
	done           chan struct{}
	conn           *websocket.Conn
	signalOnce     sync.Once
	connectionOnce sync.Once
}

func (c *webSocketRelayClient) signalClose() {
	c.signalOnce.Do(func() { close(c.done) })
}

func (c *webSocketRelayClient) closeConnection() {
	c.connectionOnce.Do(func() {
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

type webSocketRelay struct {
	mu sync.Mutex

	queueCapacity int
	clients       map[*webSocketRelayClient]struct{}
	pending       []byte
	pendingCount  int
	lastContext   []byte
	lastVersion   []byte
	statsSnapshot webSocketRelayStats
}

func newWebSocketRelay(queueCapacity int) *webSocketRelay {
	if queueCapacity < 1 {
		queueCapacity = 1
	}
	return &webSocketRelay{
		queueCapacity: queueCapacity,
		clients:       make(map[*webSocketRelayClient]struct{}),
	}
}

func encodeWebSocketBundle(packets [][]byte) ([]byte, error) {
	if len(packets) < 1 || len(packets) > webSocketMaxBundlePackets {
		return nil, errors.New("invalid websocket bundle packet count")
	}
	total := webSocketBundleHeaderSize
	for _, packet := range packets {
		if len(packet) < 1 || len(packet) > 0xffff {
			return nil, errors.New("invalid websocket bundle packet length")
		}
		total += 2 + len(packet)
	}
	if total > 512<<10 {
		return nil, errors.New("websocket bundle is too large")
	}
	bundle := make([]byte, webSocketBundleHeaderSize, total)
	binary.BigEndian.PutUint32(bundle[:4], webSocketBundleMagic)
	bundle[4] = webSocketBundleVersion
	binary.BigEndian.PutUint16(bundle[6:8], uint16(len(packets)))
	for _, packet := range packets {
		bundle = binary.BigEndian.AppendUint16(bundle, uint16(len(packet)))
		bundle = append(bundle, packet...)
	}
	return bundle, nil
}

func (r *webSocketRelay) publishPacket(packet []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheControlPacketLocked(packet)
	if len(r.clients) == 0 {
		r.pending = nil
		r.pendingCount = 0
		return
	}
	packetBytes := 2 + len(packet)
	if r.pendingCount >= webSocketMaxBundlePackets || len(r.pending)+packetBytes > webSocketMaxBundleBytes {
		r.flushLocked()
	}
	if r.pendingCount == 0 {
		r.pending = make([]byte, webSocketBundleHeaderSize, webSocketMaxBundleBytes)
		binary.BigEndian.PutUint32(r.pending[:4], webSocketBundleMagic)
		r.pending[4] = webSocketBundleVersion
	}
	r.pending = binary.BigEndian.AppendUint16(r.pending, uint16(len(packet)))
	r.pending = append(r.pending, packet...)
	r.pendingCount++
	if r.pendingCount >= webSocketMaxBundlePackets || len(r.pending) >= webSocketMaxBundleBytes {
		r.flushLocked()
	}
}

func (r *webSocketRelay) cacheControlPacketLocked(packet []byte) {
	if len(packet) < 16 || packet[0]>>4 != 4 {
		return
	}
	switch binary.BigEndian.Uint16(packet[14:16]) {
	case 1:
		r.lastContext = append(r.lastContext[:0], packet...)
	case 4:
		r.lastVersion = append(r.lastVersion[:0], packet...)
	}
}

func (r *webSocketRelay) flushLocked() {
	if r.pendingCount == 0 {
		return
	}
	binary.BigEndian.PutUint16(r.pending[6:8], uint16(r.pendingCount))
	bundle := r.pending
	packetCount := r.pendingCount
	r.pending = nil
	r.pendingCount = 0
	r.publishBundleLocked(bundle, packetCount)
}

func (r *webSocketRelay) publishBundle(bundle []byte, packetCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.publishBundleLocked(bundle, packetCount)
}

func (r *webSocketRelay) publishBundleLocked(bundle []byte, packetCount int) {
	if len(r.clients) == 0 {
		return
	}
	r.statsSnapshot.Bundles++
	r.statsSnapshot.Packets += uint64(packetCount)
	r.statsSnapshot.Bytes += uint64(len(bundle))
	for client := range r.clients {
		select {
		case client.queue <- bundle:
		default:
			// A client that cannot sustain the RF rate is disconnected explicitly.
			// Keeping it connected while skipping bundles would create an invisible
			// timestamp discontinuity and is forbidden by the transport contract.
			delete(r.clients, client)
			client.signalClose()
			if client.conn != nil {
				_ = client.conn.SetWriteDeadline(time.Now())
			}
			r.statsSnapshot.Disconnects++
		}
	}
	r.statsSnapshot.Clients = len(r.clients)
}

func (r *webSocketRelay) addClientLocked() *webSocketRelayClient {
	if len(r.clients) >= webSocketMaxClients {
		return nil
	}
	if r.pendingCount > 0 {
		r.flushLocked()
	}
	client := &webSocketRelayClient{
		queue: make(chan []byte, r.queueCapacity),
		done:  make(chan struct{}),
	}
	initial := make([][]byte, 0, 2)
	if len(r.lastContext) > 0 {
		initial = append(initial, r.lastContext)
	}
	if len(r.lastVersion) > 0 {
		initial = append(initial, r.lastVersion)
	}
	if len(initial) > 0 {
		bundle, err := encodeWebSocketBundle(initial)
		if err == nil {
			client.queue <- bundle
		}
	}
	r.clients[client] = struct{}{}
	r.statsSnapshot.Clients = len(r.clients)
	return client
}

func (r *webSocketRelay) addClientForTest() *webSocketRelayClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.addClientLocked()
}

func (r *webSocketRelay) removeClient(client *webSocketRelayClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clients[client]; !ok {
		return
	}
	delete(r.clients, client)
	client.signalClose()
	if len(r.clients) == 0 {
		r.pending = nil
		r.pendingCount = 0
	}
	r.statsSnapshot.Clients = len(r.clients)
}

func (r *webSocketRelay) stats() webSocketRelayStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	stats := r.statsSnapshot
	stats.Clients = len(r.clients)
	return stats
}

func (r *webSocketRelay) closeAll() {
	r.mu.Lock()
	clients := make([]*webSocketRelayClient, 0, len(r.clients))
	for client := range r.clients {
		delete(r.clients, client)
		client.signalClose()
		r.statsSnapshot.Disconnects++
		if client.conn != nil {
			clients = append(clients, client)
		}
	}
	r.pending = nil
	r.pendingCount = 0
	r.lastContext = nil
	r.lastVersion = nil
	r.statsSnapshot.Clients = 0
	r.mu.Unlock()
	for _, client := range clients {
		client.closeConnection()
	}
}

func (r *webSocketRelay) serveHTTP(w http.ResponseWriter, req *http.Request) {
	server := websocket.Server{
		Handshake: func(config *websocket.Config, request *http.Request) error {
			origin, err := websocket.Origin(config, request)
			if err != nil {
				return err
			}
			if !webSocketOriginAllowed(origin, request.Host) {
				return errors.New("websocket origin is not allowed")
			}
			r.mu.Lock()
			full := len(r.clients) >= webSocketMaxClients
			r.mu.Unlock()
			if full {
				return errors.New("websocket client limit reached")
			}
			config.Origin = origin
			return nil
		},
		Handler: func(conn *websocket.Conn) {
			conn.PayloadType = websocket.BinaryFrame
			conn.MaxPayloadBytes = webSocketMaxClientMessageBytes
			r.mu.Lock()
			client := r.addClientLocked()
			if client != nil {
				client.conn = conn
			}
			r.mu.Unlock()
			if client == nil {
				return
			}
			defer func() {
				r.removeClient(client)
				client.closeConnection()
			}()
			clientClosed := make(chan struct{})
			go func() {
				defer close(clientClosed)
				var message []byte
				_ = websocket.Message.Receive(conn, &message)
			}()
			for {
				select {
				case <-clientClosed:
					return
				case <-client.done:
					return
				case bundle := <-client.queue:
					if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
						return
					}
					if _, err := conn.Write(bundle); err != nil {
						return
					}
				}
			}
		},
	}
	server.ServeHTTP(w, req)
}

func webSocketOriginAllowed(origin *url.URL, requestHost string) bool {
	if origin == nil {
		return true
	}
	if strings.EqualFold(origin.Host, requestHost) {
		return true
	}
	if origin.Scheme == "https" && strings.EqualFold(origin.Host, "crt-emulator2.fuba.me") {
		return true
	}
	hostname := origin.Hostname()
	return origin.Scheme == "http" && (hostname == "localhost" || hostname == "127.0.0.1")
}
