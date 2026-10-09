package main

// Minimal RFC 6455 gateway for the official binary-datagram transport. It has
// no compression/extensions, bounds both frames and fragmented messages, and
// serializes data and control writes. No game messages become text frames.
import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type companyGatewayWS struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
	mu   sync.Mutex
}

func companyGatewayHeaderToken(value, want string) bool {
	for _, s := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}

func (g *companyGateway) serveWS(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Sec-WebSocket-Key")
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 16 || r.Header.Get("Sec-WebSocket-Version") != "13" || !companyGatewayHeaderToken(r.Header.Get("Upgrade"), "websocket") || !companyGatewayHeaderToken(r.Header.Get("Connection"), "upgrade") {
		http.Error(w, "invalid WebSocket upgrade", http.StatusBadRequest)
		return
	}
	// Each connection has its own loopback UDP address, like upstream WsGateway.
	p := g.newPeer(fmt.Sprintf("ws:%d", g.nextPeer.Add(1)), nil)
	if p == nil {
		http.Error(w, "server loading or full", http.StatusServiceUnavailable)
		return
	}
	defer p.close()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket unavailable", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return
	}
	ws := &companyGatewayWS{conn: conn, r: rw.Reader, w: rw.Writer}
	p.mu.Lock()
	if p.closed.Load() {
		p.mu.Unlock()
		_ = conn.Close()
		return
	}
	p.ws = ws
	p.mu.Unlock()
	_ = conn.SetDeadline(time.Time{})
	digest := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	_ = conn.SetWriteDeadline(time.Now().Add(4 * time.Second))
	_, err = fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(digest[:]))
	if err != nil || rw.Flush() != nil {
		return
	}
	var message []byte
	fragmenting := false
	for {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		op, final, data, err := ws.readFrame()
		if err != nil {
			_ = ws.write(8, []byte{3, 234})
			return
		}
		switch op {
		case 8:
			_ = ws.write(8, data)
			return
		case 9:
			if ws.write(10, data) != nil {
				return
			}
			continue
		case 10:
			continue
		case 2:
			if fragmenting {
				return
			}
			message = data
			fragmenting = !final
		case 0:
			if !fragmenting || len(message)+len(data) > companyGatewayPacketLimit {
				return
			}
			message = append(message, data...)
			fragmenting = !final
		default:
			_ = ws.write(8, []byte{3, 235})
			return
		}
		if !fragmenting {
			if err := p.send(message); err != nil {
				_ = ws.write(2, []byte(fmt.Sprintf("REJECT|%d|%s", multiplayerProtocol, err)))
				return
			}
			message = nil
		}
	}
}

func (ws *companyGatewayWS) readFrame() (byte, bool, []byte, error) {
	var head [2]byte
	if _, err := io.ReadFull(ws.r, head[:]); err != nil {
		return 0, false, nil, err
	}
	op, final := head[0]&15, head[0]&128 != 0
	if head[0]&0x70 != 0 || head[1]&128 == 0 {
		return 0, false, nil, fmt.Errorf("invalid WebSocket frame")
	}
	size := uint64(head[1] & 127)
	if size == 126 {
		var n [2]byte
		if _, err := io.ReadFull(ws.r, n[:]); err != nil {
			return 0, false, nil, err
		}
		size = uint64(binary.BigEndian.Uint16(n[:]))
		if size < 126 {
			return 0, false, nil, fmt.Errorf("noncanonical frame size")
		}
	} else if size == 127 {
		var n [8]byte
		if _, err := io.ReadFull(ws.r, n[:]); err != nil {
			return 0, false, nil, err
		}
		size = binary.BigEndian.Uint64(n[:])
		if size < 65536 {
			return 0, false, nil, fmt.Errorf("noncanonical frame size")
		}
	}
	if size > companyGatewayPacketLimit || (op >= 8 && (!final || size > 125)) {
		return 0, false, nil, fmt.Errorf("oversized or fragmented control frame")
	}
	var mask [4]byte
	if _, err := io.ReadFull(ws.r, mask[:]); err != nil {
		return 0, false, nil, err
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(ws.r, data); err != nil {
		return 0, false, nil, err
	}
	for i := range data {
		data[i] ^= mask[i%4]
	}
	if op == 8 && len(data) == 1 {
		return 0, false, nil, fmt.Errorf("invalid close frame")
	}
	return op, final, data, nil
}

func (ws *companyGatewayWS) write(op byte, data []byte) error {
	if len(data) > companyGatewayPacketLimit || (op >= 8 && len(data) > 125) {
		return fmt.Errorf("oversized WebSocket frame")
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	_ = ws.conn.SetWriteDeadline(time.Now().Add(4 * time.Second))
	head := []byte{128 | op}
	if len(data) < 126 {
		head = append(head, byte(len(data)))
	} else {
		head = append(head, 126, byte(len(data)>>8), byte(len(data)))
	}
	if _, err := ws.w.Write(head); err != nil {
		return err
	}
	if _, err := ws.w.Write(data); err != nil {
		return err
	}
	return ws.w.Flush()
}
