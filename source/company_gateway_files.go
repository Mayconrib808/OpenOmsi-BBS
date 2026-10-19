package main

// The native encrypted mod stream is carried without decrypting it. Native
// openOMSI owns session proofs, content hashes, quarantine and sealed storage.
// Only this gateway's fixed mods port is reachable; HTTP/admin is never proxied.
import (
	"net"
	"net/http"
	"strconv"
	"time"
)

const companyGatewayStreamLimit = 128 << 10
const companyGatewayStreamSlots = 160

func (g *companyGateway) serveFiles(w http.ResponseWriter, r *http.Request) {
	key, ok := companyGatewayWebSocketKey(w, r)
	if !ok {
		return
	}
	if !g.ready.Load() {
		http.Error(w, "server loading", http.StatusServiceUnavailable)
		return
	}
	ws, err := companyGatewayUpgrade(w, key)
	if err != nil {
		return
	}
	defer ws.conn.Close()
	ws.frameLimit, ws.writeTimeout = companyGatewayStreamLimit, 30*time.Second
	g.mu.Lock()
	if g.closed || len(g.fileStreams) >= companyGatewayStreamSlots {
		g.mu.Unlock()
		_ = ws.write(2, []byte("ERR busy\n"))
		_ = ws.write(8, []byte{3, 232})
		return
	}
	g.fileStreams[ws] = nil
	g.wg.Add(1)
	g.mu.Unlock()
	defer g.wg.Done()
	defer func() {
		g.mu.Lock()
		delete(g.fileStreams, ws)
		g.mu.Unlock()
	}()
	upstream, err := (&net.Dialer{Timeout: 4 * time.Second}).DialContext(g.ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(g.backendPort)))
	if err != nil {
		_ = ws.write(2, []byte("ERR busy\n"))
		_ = ws.write(8, []byte{3, 232})
		return
	}
	defer upstream.Close()
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	g.fileStreams[ws] = upstream
	g.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ws.conn.Close()
		buf := make([]byte, 32<<10)
		for {
			_ = upstream.SetReadDeadline(time.Now().Add(120 * time.Second))
			n, err := upstream.Read(buf)
			if n > 0 && ws.write(2, buf[:n]) != nil {
				return
			}
			if err != nil {
				_ = ws.write(8, []byte{3, 232})
				return
			}
		}
	}()
	defer func() {
		_ = upstream.Close()
		_ = ws.conn.Close()
		<-done
	}()
	fragmenting, size := false, 0
	for {
		_ = ws.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		op, final, data, err := ws.readFrame()
		if err != nil {
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
			size, fragmenting = len(data), !final
		case 0:
			if !fragmenting {
				return
			}
			size += len(data)
			fragmenting = !final
		default:
			return
		}
		if size > companyGatewayStreamLimit {
			return
		}
		_ = upstream.SetWriteDeadline(time.Now().Add(30 * time.Second))
		// TCP writes, WebSocket frames and encrypted native records have different
		// boundaries. Copy every byte, including partial writes, in its order.
		for len(data) > 0 {
			n, err := upstream.Write(data)
			if err != nil || n == 0 {
				return
			}
			data = data[n:]
		}
	}
}
