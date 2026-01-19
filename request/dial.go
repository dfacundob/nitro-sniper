package request

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Helper for tls.Conn claimer

// Pool of claimDialer
type claimDialerConnPool struct {
	mux  sync.Mutex
	pool []*claimDialer
	size int
	host string
}

func newClaimDialerConnPool(size int, host string, tlsConfig func() *tls.Config) *claimDialerConnPool {
	pool := make([]*claimDialer, size)

	for i := 0; i < size; i++ {
		pool[i] = newClaimDialer(host, tlsConfig)
		pool[i].EstablishConnection()
	}

	return &claimDialerConnPool{
		pool: pool,
		size: size,
		host: host,
	}
}

func (p *claimDialerConnPool) Get() (*claimDialer, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	for _, conn := range p.pool {
		// conn.mux.Lock()
		if !conn.IsBusy.Load() && !conn.ShouldRevive.Load() {
			conn.IsBusy.Store(true)
			// conn.mux.Unlock()
			return conn, nil
		}
		// conn.mux.Unlock()
	}

	return nil, fmt.Errorf("connection pool is full")
}

func (p *claimDialerConnPool) GetForPing() []*claimDialer {
	p.mux.Lock()
	defer p.mux.Unlock()

	var ret []*claimDialer
	timeNow := time.Now()
	for _, conn := range p.pool {
		if !conn.IsBusy.Load() && (timeNow.Sub(conn.LastPing) > time.Duration(time.Second*20) || conn.ShouldRevive.Load()) {
			conn.IsBusy.Store(true)
			ret = append(ret, conn)
		}
	}

	return ret
}

func (p *claimDialerConnPool) Release(dialer *claimDialer) {
	// dialer.mux.Lock()
	dialer.IsBusy.Store(false)
	// dialer.mux.Unlock()
}

type claimDialer struct {
	mux       sync.Mutex
	conn      *tls.Conn
	host      string
	tlsConfig func() *tls.Config
	LastPing  time.Time

	// this is not handled by claimDialer
	IsBusy atomic.Bool

	// if we should force revive the connection
	ShouldRevive atomic.Bool
}

func newClaimDialer(host string, tlsConfig func() *tls.Config) *claimDialer {
	return &claimDialer{
		host:      host,
		tlsConfig: tlsConfig,
	}
}

func (c *claimDialer) GetLastPing() time.Time {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.LastPing
}

func (c *claimDialer) createConn() (*tls.Conn, error) {
	return tls.Dial("tcp", c.host+":443", c.tlsConfig())
}

// assuming this gets called when the mutex is locked
func (c *claimDialer) reviveConnection() error {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}

	var err error = nil
	c.conn, err = c.createConn()
	if err != nil {
		c.conn = nil
		return err
	}

	c.ShouldRevive.Store(false)
	return nil
}

func (c *claimDialer) EstablishConnection() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	return c.reviveConnection()
}

func (c *claimDialer) MakeRequest(requestData []byte) (*http.Response, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.conn == nil {
		if err := c.reviveConnection(); err != nil {
			return nil, fmt.Errorf("error making request: conn nil | failed to revive connection: %v", err)
		}
	}

	var retries int = 0
retryRequest:
	c.conn.SetDeadline(time.Now().Add(time.Minute))
	_, err := c.conn.Write(requestData)
	if err != nil {
		if err2 := c.reviveConnection(); err2 != nil {
			return nil, fmt.Errorf("error writing to server: %v | failed to revive connection: %v", err, err2)
		}

		if retries == 0 {
			retries++
			goto retryRequest
		}

		return nil, fmt.Errorf("error writing to server: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(c.conn), nil)
	if err != nil {
		return nil, fmt.Errorf("error reading from server: %v", err)
	}

	c.LastPing = time.Now()
	return resp, nil
}
