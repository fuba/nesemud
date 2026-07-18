// SPDX-License-Identifier: MIT

package rfstream

import (
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestEncodeWebSocketBundleMatchesCRTNCB1(t *testing.T) {
	first := []byte{1, 2, 3}
	second := []byte{4, 5}
	bundle, err := encodeWebSocketBundle([][]byte{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.BigEndian.Uint32(bundle[:4]); got != webSocketBundleMagic {
		t.Fatalf("bundle magic=%#x", got)
	}
	if bundle[4] != 1 || bundle[5] != 0 || binary.BigEndian.Uint16(bundle[6:8]) != 2 {
		t.Fatalf("bundle header=%x", bundle[:8])
	}
	if got := bundle[8:]; string(got) != string([]byte{0, 3, 1, 2, 3, 0, 2, 4, 5}) {
		t.Fatalf("bundle payload=%x", got)
	}
}

func TestWebSocketRelayDisconnectsInsteadOfSilentlySkippingFullClient(t *testing.T) {
	relay := newWebSocketRelay(1)
	client := relay.addClientForTest()
	relay.publishBundle([]byte("first"), 1)
	relay.publishBundle([]byte("second"), 1)

	select {
	case <-client.done:
	default:
		t.Fatal("slow client was not disconnected after its lossless queue filled")
	}
	stats := relay.stats()
	if stats.Clients != 0 || stats.Disconnects != 1 {
		t.Fatalf("relay stats=%+v", stats)
	}
}

func TestWebSocketRelayServesCrossOriginBinaryNCB1(t *testing.T) {
	relay := newWebSocketRelay(4)
	server := httptest.NewServer(http.HandlerFunc(relay.serveHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := websocket.Dial(wsURL, "", "https://crt-emulator2.fuba.me")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}

	contextPacket := EncodeContextPacket(1, 0, timestamp{}, DefaultRFCenterHz, true, false)
	dataPacket, err := EncodeDataPacket(1, 0, timestamp{}, []IQSample{{I: 1, Q: 2}})
	if err != nil {
		t.Fatal(err)
	}
	relay.publishPacket(contextPacket)
	relay.publishPacket(dataPacket)
	relay.mu.Lock()
	relay.flushLocked()
	relay.mu.Unlock()

	var message []byte
	if err := websocket.Message.Receive(conn, &message); err != nil {
		t.Fatal(err)
	}
	if got := binary.BigEndian.Uint32(message[:4]); got != webSocketBundleMagic {
		t.Fatalf("message magic=%#x", got)
	}
	if got := binary.BigEndian.Uint16(message[6:8]); got != 2 {
		t.Fatalf("message packet count=%d", got)
	}
}

func TestWebSocketRelayRejectsUntrustedBrowserOrigin(t *testing.T) {
	relay := newWebSocketRelay(1)
	server := httptest.NewServer(http.HandlerFunc(relay.serveHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	if conn, err := websocket.Dial(wsURL, "", "https://example.invalid"); err == nil {
		_ = conn.Close()
		t.Fatal("untrusted browser origin was accepted")
	}
}

func TestWebSocketRelayRemovesClientAfterCloseFrame(t *testing.T) {
	relay := newWebSocketRelay(1)
	server := httptest.NewServer(http.HandlerFunc(relay.serveHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := websocket.Dial(wsURL, "", "https://crt-emulator2.fuba.me")
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for relay.stats().Clients != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if clients := relay.stats().Clients; clients != 0 {
		t.Fatalf("clients=%d after websocket close", clients)
	}
}

func TestWebSocketRelayDiscardsPartialBundleAfterLastClientLeaves(t *testing.T) {
	relay := newWebSocketRelay(1)
	relay.mu.Lock()
	client := relay.addClientLocked()
	relay.mu.Unlock()
	relay.publishPacket([]byte{0x18, 1, 2, 3})
	if relay.pendingCount != 1 {
		t.Fatalf("pending packets=%d, want 1", relay.pendingCount)
	}
	relay.removeClient(client)
	relay.publishPacket([]byte{0x18, 4, 5, 6})
	if relay.pendingCount != 0 || len(relay.pending) != 0 {
		t.Fatalf("stale partial bundle remains: packets=%d bytes=%d", relay.pendingCount, len(relay.pending))
	}
}

func TestWebSocketRelayCloseAllDisconnectsClientsAndClearsCachedPackets(t *testing.T) {
	relay := newWebSocketRelay(1)
	relay.lastContext = []byte{1}
	relay.lastVersion = []byte{2}
	client := relay.addClientForTest()
	relay.closeAll()
	select {
	case <-client.done:
	default:
		t.Fatal("client remained connected after relay close")
	}
	if stats := relay.stats(); stats.Clients != 0 {
		t.Fatalf("relay stats=%+v", stats)
	}
	if len(relay.lastContext) != 0 || len(relay.lastVersion) != 0 {
		t.Fatal("relay retained stale control packets after close")
	}
}

func TestWebSocketRelayCloseAllClosesNetworkConnection(t *testing.T) {
	relay := newWebSocketRelay(1)
	server := httptest.NewServer(http.HandlerFunc(relay.serveHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := websocket.Dial(wsURL, "", "https://crt-emulator2.fuba.me")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	relay.closeAll()
	var message []byte
	if err := websocket.Message.Receive(conn, &message); err == nil {
		t.Fatal("network connection remained readable after relay close")
	}
}

func TestWebSocketRelayRejectsLargeClientMessage(t *testing.T) {
	relay := newWebSocketRelay(1)
	server := httptest.NewServer(http.HandlerFunc(relay.serveHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := websocket.Dial(wsURL, "", "https://crt-emulator2.fuba.me")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := websocket.Message.Send(conn, make([]byte, webSocketMaxClientMessageBytes+1)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for relay.stats().Clients != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if clients := relay.stats().Clients; clients != 0 {
		t.Fatalf("clients=%d after oversized inbound message", clients)
	}
}

func TestWebSocketRelayRejectsClientApplicationData(t *testing.T) {
	relay := newWebSocketRelay(1)
	server := httptest.NewServer(http.HandlerFunc(relay.serveHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := websocket.Dial(wsURL, "", "https://crt-emulator2.fuba.me")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := websocket.Message.Send(conn, []byte{1}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for relay.stats().Clients != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if clients := relay.stats().Clients; clients != 0 {
		t.Fatalf("clients=%d after forbidden inbound application data", clients)
	}
}

func TestNewWebSocketClientStartsAfterExistingPartialBundle(t *testing.T) {
	relay := newWebSocketRelay(4)
	firstClient := relay.addClientForTest()
	relay.publishPacket([]byte{0x18, 1, 2, 3})
	secondClient := relay.addClientForTest()

	select {
	case bundle := <-firstClient.queue:
		if count := binary.BigEndian.Uint16(bundle[6:8]); count != 1 {
			t.Fatalf("first-client packet count=%d", count)
		}
	default:
		t.Fatal("existing partial bundle was not flushed before adding a client")
	}
	select {
	case bundle := <-secondClient.queue:
		t.Fatalf("new client received pre-connection bundle %x", bundle)
	default:
	}
}
