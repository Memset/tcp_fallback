// tcp_fallback, a simple TCP proxy
// Copyright (C) 2012 by Memset Ltd. http://www.memset.com/
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
//
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net"
	"os"
	"path"
	"sync"
	"time"
)

// Globals
var (
	timeout       = flag.Duration("timeout", time.Second*5, "Timeout for backend connection")
	probe_delay   = flag.Duration("probe-delay", time.Second*30, "Interval to delay probes after backend error")
	useSyslog     = flag.Bool("syslog", false, "Use Syslog for logging")
	debug         = flag.Bool("debug", false, "Enable verbose logging")
	statsInterval = flag.Duration("stats", time.Minute*15, "Interval to log stats")
	me            = path.Base(os.Args[0])
	version       = "0.1"
)

// Backends stats
type Backend struct {
	address   string
	timestamp time.Time
	requests  int
	errors    int
}

// A list of backends
type Backends []*Backend

// Log the contents if debug
func logDebug(text string, params ...interface{}) {
	if !*debug {
		return
	}
	line := fmt.Sprintf(text, params...)
	log.Printf("DEBUG: %s", line)
}

// Copy one side of the socket, doing a half close when it has
// finished
func copy_half(dst, src *net.TCPConn, wg *sync.WaitGroup) {
	defer wg.Done()
	_, err := io.Copy(dst, src)
	if err != nil {
		log.Printf("Error: %s", err)
	}
	dst.CloseWrite()
	src.CloseRead()
}

// NewBackends creates a Backends structure from the remote
// addresses passed in
func NewBackends(remoteAddrs []string) Backends {
	backends := make(Backends, len(remoteAddrs))
	for i, remoteAddr := range remoteAddrs {
		backends[i] = &Backend{address: remoteAddr, timestamp: time.Now()}
	}
	return backends
}

// Forward the incoming TCP connection to one of the remote addresses
func forward(local *net.TCPConn, remote *net.TCPConn) {
	var wg sync.WaitGroup
	wg.Add(2)
	logDebug("<%s> Start transfer %s to %s", remote.RemoteAddr(), local.LocalAddr(), remote.LocalAddr())
	go copy_half(local, remote, &wg)
	go copy_half(remote, local, &wg)
	wg.Wait()
	logDebug("<%s> Finished transfer from %s to %s done", remote.RemoteAddr(), local.LocalAddr(), remote.LocalAddr())
}

// connect attempts to connect to a backend
func (backends Backends) connect() *net.TCPConn {
	var remote *net.TCPConn
	for _, backend := range backends {
		if backend.timestamp.After(time.Now()) {
			logDebug("<%s> Delayed probe (next: %s)", backend.address, backend.timestamp)
			continue
		}

		remote_conn, err := net.DialTimeout("tcp", backend.address, *timeout)
		if err == nil {
			remote = remote_conn.(*net.TCPConn)

			// refresh last time it was used
			backend.timestamp = time.Now()
			backend.requests += 1
			break
		}

		log.Printf("Failed to connect to backend %s: %s", backend.address, err)
		logDebug("err=%q, remote=%q", err, remote_conn)

		// don't check that backend for probe_delay seconds
		backend.timestamp = time.Now().Add(*probe_delay)
		backend.errors += 1
	}
	if remote == nil {
		// next probe try all backends
		for _, backend := range backends {
			backend.timestamp = time.Now()
		}
	}
	return remote
}

// logs_stats dump backends stats in the log
func (backends Backends) log_stats() {
	for _, backend := range backends {
		log.Printf("STATS: <%s> requests=%d errors=%d last=%s", backend.address, backend.requests, backend.errors, backend.timestamp)
	}
}

// usage prints a help message
func usage() {
	fmt.Fprintf(os.Stderr,
	    "%s ver %s\n\n"+
		"Usage: %s [flags] <local-address:port> [<remote-address:port>]+\n\n"+
			"flags:\n\n",
		me, version, me)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n")
}

// Main script
func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	if *useSyslog {
		w, _ := syslog.New(syslog.LOG_INFO, me)
		log.SetFlags(0)
		log.SetOutput(w)
	}

	localAddr := flag.Args()[0]
	backends := NewBackends(flag.Args()[1:])

	// Print the stats every statsInterval
	go func() {
		ch := time.Tick(*statsInterval)
		for {
			<-ch
			backends.log_stats()
		}
	}()

	// Open the main listening socket
	local, err := net.Listen("tcp", localAddr)
	if local == nil {
		log.Fatalf("Failed to open listening socket: %s", err)
	}

	// Main loop accepting connections
	log.Printf("Starting, listening on %s", localAddr)
	for {
		conn, err := local.Accept()
		if err != nil {
			log.Fatalf("Accept failed: %s", err)
		}
		if remote := backends.connect(); remote != nil {
			go forward(conn.(*net.TCPConn), remote)
		} else {
			log.Printf("Failed to connect to any backend")
			conn.Close()
		}
	}
}
