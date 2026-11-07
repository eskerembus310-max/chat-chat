package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type wsConn struct {
	conn net.Conn
	out  chan []byte
	uid  int
}

var (
	online  = map[int]map[*wsConn]bool{} // под mu
	wsCount int64
)

func isOnline(id int) bool { return len(online[id]) > 0 }

func wsFrame(op byte, data []byte) []byte {
	n := len(data)
	var head []byte
	switch {
	case n < 126:
		head = []byte{0x80 | op, byte(n)}
	case n < 65536:
		head = []byte{0x80 | op, 126, 0, 0}
		binary.BigEndian.PutUint16(head[2:], uint16(n))
	default:
		head = make([]byte, 10)
		head[0], head[1] = 0x80|op, 127
		binary.BigEndian.PutUint64(head[2:], uint64(n))
	}
	return append(head, data...)
}

// Все функции рассылки вызываются при захваченном mu.
func sendTo(userID int, payload any) {
	set := online[userID]
	if len(set) == 0 {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fr := wsFrame(0x1, b)
	for c := range set {
		select {
		case c.out <- fr:
		default:
			_ = c.conn.Close()
		}
	}
}
func broadcastChat(chatID int, payload any, except int) {
	for _, id := range memberIDs(chatID) {
		if id != except {
			sendTo(id, payload)
		}
	}
}
func broadcastAll(payload any) {
	for id := range online {
		sendTo(id, payload)
	}
}
func notifyPresence(userID int) {
	u := userByID(userID)
	var ls int64
	if u != nil {
		ls = u.LastSeen
	}
	for _, p := range peersOf(userID, false) {
		sendTo(p, map[string]any{"t": "presence", "userId": userID, "online": isOnline(userID), "lastSeen": ls})
	}
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || r.Header.Get("Sec-WebSocket-Key") == "" {
		http.Error(w, "bad request", 400)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "no hijack", 500)
		return
	}
	conn, brw, err := hj.Hijack()
	if err != nil {
		return
	}
	sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	_, _ = brw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n")
	if brw.Flush() != nil {
		conn.Close()
		return
	}
	c := &wsConn{conn: conn, out: make(chan []byte, 64)}
	atomic.AddInt64(&wsCount, 1)
	done := make(chan struct{})
	go func() { // писатель
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case b := <-c.out:
				_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
				if _, err := conn.Write(b); err != nil {
					conn.Close()
					return
				}
			case <-tick.C:
				_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
				if _, err := conn.Write(wsFrame(0x9, nil)); err != nil {
					conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()
	defer func() {
		close(done)
		conn.Close()
		atomic.AddInt64(&wsCount, -1)
		if c.uid != 0 {
			mu.Lock()
			if set := online[c.uid]; set != nil {
				delete(set, c)
				if len(set) == 0 {
					delete(online, c.uid)
					if u := userByID(c.uid); u != nil {
						u.LastSeen = now()
					}
					notifyPresence(c.uid)
					save()
				}
			}
			mu.Unlock()
		}
	}()
	rd := brw.Reader
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second)) // на вход даём 10 секунд
	var parts []byte
	for {
		h := make([]byte, 2)
		if _, err := io.ReadFull(rd, h); err != nil {
			return
		}
		fin, op, masked := h[0]&0x80 != 0, h[0]&0x0f, h[1]&0x80 != 0
		ln := int(h[1] & 0x7f)
		if ln == 126 {
			e := make([]byte, 2)
			if _, err := io.ReadFull(rd, e); err != nil {
				return
			}
			ln = int(binary.BigEndian.Uint16(e))
		} else if ln == 127 {
			e := make([]byte, 8)
			if _, err := io.ReadFull(rd, e); err != nil {
				return
			}
			v := binary.BigEndian.Uint64(e)
			if v > 65536 {
				return
			}
			ln = int(v)
		}
		if !masked || ln > 65536 {
			return
		}
		mask := make([]byte, 4)
		if _, err := io.ReadFull(rd, mask); err != nil {
			return
		}
		pl := make([]byte, ln)
		if _, err := io.ReadFull(rd, pl); err != nil {
			return
		}
		for i := range pl {
			pl[i] ^= mask[i&3]
		}
		if c.uid != 0 {
			_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		}
		switch op {
		case 0x8:
			select {
			case c.out <- wsFrame(0x8, nil):
			default:
			}
			return
		case 0x9:
			select {
			case c.out <- wsFrame(0xa, pl):
			default:
			}
			continue
		case 0xa:
			continue
		}
		parts = append(parts, pl...)
		if len(parts) > 65536 {
			return
		}
		if !fin {
			continue
		}
		var m map[string]any
		err := json.Unmarshal(parts, &m)
		parts = nil
		if err != nil || m == nil {
			continue
		}
		t, _ := m["t"].(string)
		if c.uid == 0 {
			if t != "auth" {
				return
			}
			tok, _ := m["token"].(string)
			mu.Lock()
			uid, okk := db.Sessions[sha(tok)]
			if !okk {
				mu.Unlock()
				c.out <- wsFrame(0x1, []byte(`{"t":"auth_fail"}`))
				time.Sleep(200 * time.Millisecond)
				return
			}
			c.uid = uid
			set := online[uid]
			first := len(set) == 0
			if set == nil {
				set = map[*wsConn]bool{}
				online[uid] = set
			}
			set[c] = true
			c.out <- wsFrame(0x1, []byte(`{"t":"ready","userId":`+itoa(uid)+`}`))
			if first {
				notifyPresence(uid)
			}
			mu.Unlock()
			_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			continue
		}
		if t == "typing" {
			if n, k := toInt(m["chat"]); k {
				mu.Lock()
				if isMember(n, c.uid) {
					broadcastChat(n, map[string]any{"t": "typing", "chat": n, "userId": c.uid}, c.uid)
				}
				mu.Unlock()
			}
		}
	}
}

func itoa(n int) string { return fmtInt(n) }
func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

var _ = bufio.NewReader
