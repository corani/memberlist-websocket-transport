package main

import (
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Adapter struct {
	*websocket.Conn
	wg            sync.WaitGroup
	currentReader io.Reader
	mutex         sync.Mutex
}

func connAdapter(c *websocket.Conn) *Adapter {
	res := &Adapter{
		Conn:  c,
		wg:    sync.WaitGroup{},
		mutex: sync.Mutex{},
	}

	res.wg.Add(1)

	return res
}

func (a *Adapter) Close() error {
	defer a.wg.Done()

	return a.Conn.Close()
}

func (a *Adapter) Wait() {
	a.wg.Wait()
}

func (a *Adapter) getReader() (io.Reader, error) {
	if a.currentReader == nil {
		_, r, err := a.NextReader()
		if err != nil {
			return nil, err
		}

		a.currentReader = r
	}

	return a.currentReader, nil
}

func (a *Adapter) Read(b []byte) (int, error) {
	r, err := a.getReader()
	if err != nil {
		return 0, err
	}

	n, err := r.Read(b)
	if err != nil {
		a.currentReader = nil
	}

	return n, err
}

func (a *Adapter) Write(b []byte) (int, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	w, err := a.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer w.Close()

	n, err := w.Write(b)
	if err != nil {
		return 0, err
	}

	return n, err
}

func (a *Adapter) SetDeadline(t time.Time) error {
	// return a.Conn.UnderlyingConn().SetDeadline(t)
	return nil
}
