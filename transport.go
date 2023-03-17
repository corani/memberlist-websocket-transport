package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/memberlist"
)

const MemberlistConnectionHeader = "X-Memberlist-Connection"

const (
	TypeStream = "Stream"
	TypePacket = "Packet"
)

type Option func(*Transport)

func WithEndpoint(endpoint string) Option {
	return func(t *Transport) {
		t.Route = endpoint
	}
}

type Transport struct {
	ip       string
	logger   *log.Logger
	Port     int
	Route    string
	packetCh chan *memberlist.Packet
	streamCh chan net.Conn
	shutdown int32
	wg       sync.WaitGroup
	conn     map[string]*LockedConn
}

type LockedConn struct {
	mutex sync.Mutex
	conn  *websocket.Conn
}

// NOTE(daniel): "static assert" to ensure HTTPTransport implements the memberlist.Transport
// interface.
var _ memberlist.Transport = (*Transport)(nil)

func NewTransport(ip string, port int, options ...Option) *Transport {
	// TODO(daniel): take in the base http transport?
	t := &Transport{
		ip:       ip,
		Port:     port,
		Route:    "/_memberlist",
		packetCh: make(chan *memberlist.Packet),
		streamCh: make(chan net.Conn),
		shutdown: 0,
		wg:       sync.WaitGroup{},
		conn:     make(map[string]*LockedConn),
	}

	for _, option := range options {
		option(t)
	}

	return t
}

func (t *Transport) Handler(w http.ResponseWriter, r *http.Request) {
	if s := atomic.LoadInt32(&t.shutdown); s == 1 {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "service is shutting down")
	}

	t.wg.Add(1)
	defer t.wg.Done()

	upgrader := websocket.Upgrader{
		EnableCompression: true,
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		t.logger.Printf("[WARN] transport: websocket upgrade failed: %v", err)

		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, err.Error())
	}
	defer c.Close()

	switch r.Header.Get(MemberlistConnectionHeader) {
	case TypeStream:
		t.handleStreamConnection(c)
	case TypePacket:
		t.handlePacketConnection(c)
	}
}

func (t *Transport) handleStreamConnection(c *websocket.Conn) {
	adapter := connAdapter(c)

	t.streamCh <- adapter

	adapter.Wait()
}

func (t *Transport) handlePacketConnection(c *websocket.Conn) {
	closeWebsocket := func(err error) {
		if err := c.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error()),
			time.Now().Add(time.Second),
		); err != nil {
			t.logger.Printf("[WARN] transport: error closing websocket: %v", err)
		}
	}

	// TODO(daniel): bi-directional connections.
	for {
		if s := atomic.LoadInt32(&t.shutdown); s == 1 {
			closeWebsocket(fmt.Errorf("service is shutting down"))

			break
		}

		_, r, err := c.NextReader()
		if err != nil {
			closeWebsocket(err)

			break
		}

		buf := new(bytes.Buffer)
		now := time.Now()

		if _, err := io.Copy(buf, r); err != nil {
			closeWebsocket(err)

			break
		}

		t.packetCh <- &memberlist.Packet{
			Buf:       buf.Bytes(),
			From:      c.RemoteAddr(),
			Timestamp: now,
		}
	}
}

func (t *Transport) FinalAdvertiseAddr(string, int) (net.IP, int, error) {
	if t.ip != "0.0.0.0" {
		parsedIP := net.ParseIP(t.ip)

		return parsedIP, t.Port, nil
	}

	ip, err := sockaddr.GetPrivateIP()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get interface address: %v", err)
	}

	parsedIP := net.ParseIP(ip)

	return parsedIP, t.Port, nil
}

func (t *Transport) WriteTo(b []byte, addr string) (time.Time, error) {
	if _, ok := t.conn[addr]; !ok {
		c, err := t.connect(addr, TypePacket, time.Minute)
		if err != nil {
			return time.Time{}, err
		}

		t.conn[addr] = &LockedConn{
			conn:  c,
			mutex: sync.Mutex{},
		}
	}

	c := t.conn[addr]
	c.mutex.Lock()
	defer c.mutex.Unlock()

	w, err := c.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return time.Time{}, err
	}
	defer w.Close()

	_, err = w.Write(b)

	return time.Now(), err
}

func (t *Transport) WriteToAddress(b []byte, addr memberlist.Address) (time.Time, error) {
	return t.WriteTo(b, addr.Addr)
}

func (t *Transport) PacketCh() <-chan *memberlist.Packet {
	return t.packetCh
}

func (t *Transport) StreamCh() <-chan net.Conn {
	return t.streamCh
}

func (t *Transport) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	c, err := t.connect(addr, TypeStream, timeout)
	if err != nil {
		return nil, err
	}

	return connAdapter(c), nil
}

func (t *Transport) DialAddressTimeout(addr memberlist.Address, timeout time.Duration) (net.Conn, error) {
	return t.DialTimeout(addr.Addr, timeout)
}

func (t *Transport) Shutdown() error {
	if !atomic.CompareAndSwapInt32(&t.shutdown, 0, 1) {
		// already shutting down
		return nil
	}

	for _, v := range t.conn {
		v.mutex.Lock()
		v.conn.Close()
		v.mutex.Unlock()
	}

	return nil
}

func (t *Transport) connect(addr string, kind string, timeout time.Duration) (*websocket.Conn, error) {
	// TODO(daniel): connection pool, instead of creating a connection for each write!
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: timeout,
	}

	c, _, err := dialer.Dial(fmt.Sprintf("ws://%s%s", addr, t.Route), http.Header{
		MemberlistConnectionHeader: []string{kind},
	})

	return c, err
}
