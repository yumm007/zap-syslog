// Copyright (c) 2017 Timon Wong
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zapsyslog

import (
	"go.uber.org/atomic"
	"net"
	"strings"

	"go.uber.org/zap/zapcore"
)

var (
	_ zapcore.WriteSyncer = &ConnSyncer{}
)

// ConnSyncer describes connection sink for syslog.
type ConnSyncer struct {
	enabled  atomic.Bool
	hostAsIP bool
	network  string
	raddr    string
	localIP  string
	conn     net.Conn
}

// NewConnSyncer returns a new conn sink for syslog.
func NewConnSyncer(network, raddr string, HostAsIP bool) (*ConnSyncer, error) {
	s := &ConnSyncer{
		network:  network,
		raddr:    raddr,
		hostAsIP: HostAsIP,
	}

	err := s.connect()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *ConnSyncer) Enable(network, raddr string) {
	s.network = network
	s.raddr = raddr
	s.enabled.Store(true)
	s.close()
}

func (s *ConnSyncer) Disable() {
	s.enabled.Store(false)
}

func (s *ConnSyncer) close() {
	if s.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		_ = s.conn.Close()
		s.conn = nil
	}
}

// connect makes a connection to the syslog server.
func (s *ConnSyncer) connect() error {
	if s.enabled.Load() == false {
		return nil
	}
	s.close()

	var c net.Conn
	c, err := net.Dial(s.network, s.raddr)
	if err != nil {
		return err
	}

	s.conn = c

	// update local addr
	localaddr := c.LocalAddr().String()
	ipport := strings.Split(localaddr, ":")
	if len(ipport) >= 1 {
		s.localIP = ipport[0]
	} else {
		s.localIP = "-"
	}

	return nil
}

// Write writes to syslog with retry.
func (s *ConnSyncer) Write(p []byte) (n int, err error) {
	if s.enabled.Load() == false {
		s.close()
		return n, nil
	}

	if s.conn != nil {
		// replace host name to ip addr
		// <134>1 2022-02-10T23:52:22.556653+08:00 yumingmingdeMacBook-Pro.local season 9229 - - {"caller":"logger/logger.go:31","msg":"="}
		if s.hostAsIP {
			log := strings.SplitN(string(p), " ", 4)
			if len(log) >= 3 {
				log[2] = s.localIP
				p = []byte(strings.Join(log, " "))
			}
		}
		if n, err := s.conn.Write(p); err == nil {
			return n, err
		}
	}
	if err := s.connect(); err != nil {
		return 0, err
	}

	return s.conn.Write(p)
}

// Sync implements zapcore.WriteSyncer interface.
func (s *ConnSyncer) Sync() error {
	return nil
}
