package mm

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// wsConn is the client side of a WebSocket (RFC 6455), enough for the
// server's JSON text messages.
type wsConn struct {
	rw  io.ReadWriteCloser
	br  *bufio.Reader
	wmu sync.Mutex
}

const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
	maxMessage     = 32 << 20
)

// http1Only returns a client like hc that never speaks HTTP/2, which has
// no Upgrade.
func http1Only(hc *http.Client) *http.Client {
	t, ok := hc.Transport.(*http.Transport)
	if hc.Transport == nil {
		t, ok = http.DefaultTransport.(*http.Transport), true
	}
	if !ok {
		return hc
	}
	t = t.Clone()
	var p http.Protocols
	p.SetHTTP1(true)
	t.Protocols = &p
	c := *hc
	c.Transport = t
	return &c
}

func dialWS(ctx context.Context, hc *http.Client, rawURL string, header http.Header) (*wsConn, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	k := base64.StdEncoding.EncodeToString(key)
	for name, v := range header {
		req.Header[name] = v
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", k)
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		defer res.Body.Close()
		if res.StatusCode == http.StatusUnauthorized {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("websocket handshake: %s", res.Status)
	}
	sum := sha1.Sum([]byte(k + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if res.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) {
		res.Body.Close()
		return nil, errors.New("websocket handshake: bad accept key")
	}
	rw, ok := res.Body.(io.ReadWriteCloser)
	if !ok {
		res.Body.Close()
		return nil, errors.New("websocket handshake: connection not writable")
	}
	return &wsConn{rw: rw, br: bufio.NewReader(rw)}, nil
}

// ReadMessage returns the next text or binary message, answering pings on
// the way. onFrame runs for every frame, pings included, as a sign of life.
func (c *wsConn) ReadMessage(onFrame func()) ([]byte, error) {
	var msg []byte
	for {
		fin, op, payload, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		if onFrame != nil {
			onFrame()
		}
		switch op {
		case opPing:
			if err := c.write(opPong, payload); err != nil {
				return nil, err
			}
			continue
		case opPong:
			continue
		case opClose:
			_ = c.write(opClose, payload)
			return nil, io.EOF
		case opText, opBinary, opContinuation:
			msg = append(msg, payload...)
			if len(msg) > maxMessage {
				return nil, errors.New("websocket message too large")
			}
			if fin {
				return msg, nil
			}
		default:
			return nil, fmt.Errorf("websocket: unknown opcode %d", op)
		}
	}
}

func (c *wsConn) readFrame() (fin bool, op byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.br, h[:]); err != nil {
		return
	}
	fin, op = h[0]&0x80 != 0, h[0]&0x0F
	masked := h[1]&0x80 != 0
	n := uint64(h[1] & 0x7F)
	switch n {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(c.br, b[:]); err != nil {
			return
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(c.br, b[:]); err != nil {
			return
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > maxMessage {
		err = errors.New("websocket frame too large")
		return
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(c.br, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.br, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return
}

// WriteText sends a text message. It is safe to call from any goroutine.
func (c *wsConn) WriteText(p []byte) error { return c.write(opText, p) }

func (c *wsConn) write(op byte, p []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	frame := make([]byte, 0, 14+len(p))
	frame = append(frame, 0x80|op)
	switch n := len(p); {
	case n < 126:
		frame = append(frame, 0x80|byte(n))
	case n <= 0xFFFF:
		frame = append(frame, 0x80|126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(n))
	default:
		frame = append(frame, 0x80|127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(n))
	}
	var mask [4]byte
	_, _ = rand.Read(mask[:])
	frame = append(frame, mask[:]...)
	start := len(frame)
	frame = append(frame, p...)
	for i := range p {
		frame[start+i] ^= mask[i%4]
	}
	_, err := c.rw.Write(frame)
	return err
}

func (c *wsConn) Close() error { return c.rw.Close() }
