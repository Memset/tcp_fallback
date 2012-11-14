// A simple MySQL proxy
//
// Connect to the addresses in order - if one works use it
//
// On any sort of error then close the connection so the client can retry

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
	use_syslog    = flag.Bool("syslog", false, "Use Syslog for logging")
	debug         = flag.Bool("debug", false, "Enable verbose logging")
	statsInterval = flag.Duration("stats", time.Minute*15, "Interval to log stats")
	me            = path.Base(os.Args[0])
)

// Backends stats
type Backend struct {
	timestamp time.Time
	requests  int
	errors    int
}

// A list of backends
type Backends map[string]Backend

// Log the contents if debug
func logDebug(text string, params ...interface{}) {
	if !*debug {
		return
	}
	log.Printf(text, params...)
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

// Forward the incoming TCP connection to one of the remote addresses
func forward(local *net.TCPConn, remote *net.TCPConn) {
	var wg sync.WaitGroup
	wg.Add(2)
	logDebug("DEBUG: <%s> Start transfer %s to %s", remote.RemoteAddr(), local.LocalAddr(), remote.LocalAddr())
	go copy_half(local, remote, &wg)
	go copy_half(remote, local, &wg)
	wg.Wait()
	logDebug("DEBUG: <%s> Finished transfer from %s to %s done", remote.RemoteAddr(), local.LocalAddr(), remote.LocalAddr())
}

// connect attempts to connect to a backend
func (backends Backends) connect() *net.TCPConn {
	var remote *net.TCPConn
	for address, backend := range backends {

		if backend.timestamp.After(time.Now()) {
			logDebug("DEBUG: <%s> Delayed probe (next: %s)", address, backend.timestamp)
			continue
		}

		remote_conn, err := net.DialTimeout("tcp", address, *timeout)
		if err == nil {
			remote = remote_conn.(*net.TCPConn)

			// refresh last time it was used
			backend.timestamp = time.Now()
			backend.requests += 1
			backends[address] = backend
			break
		}

		log.Printf("Failed to connect to backend %s: %s", address, err)
		logDebug("DEBUG: err=%q, remote=%q", err, remote_conn)

		// don't check that backend for probe_delay seconds
		backend.timestamp = time.Now().Add(*probe_delay)
		backend.errors += 1
		backends[address] = backend
	}
	if remote == nil {
		// next try probe all backends
		for address, backend := range backends {
			backend.timestamp = time.Now()
			backends[address] = backend
		}
	}
	return remote
}

// logs_stats dump backends stats in the log
func (backends Backends) log_stats() {
	for address, backend := range backends {
		log.Printf("STATS: <%s>, requests=%s errors=%s last=%s", address, backend.requests, backend.errors, backend.timestamp)
	}
}

// usage prints a help message
func usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [flags] <local-address:port> [<remote-address:port>]+\n\n"+
			"flags:\n\n",
		me)
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

	if *use_syslog {
		w, _ := syslog.New(syslog.LOG_INFO, me)
		log.SetFlags(0)
		log.SetOutput(w)
	}

	localAddr := flag.Args()[0]

	backends := Backends{}
	for _, remoteAddr := range flag.Args()[1:] {
		backends[remoteAddr] = Backend{time.Now(), 0, 0}
	}

	// Print the stats every statsInterval
	go func() {
		ch := time.Tick(*statsInterval)
		for {
			<-ch
			backends.log_stats()
		}
	}()

	local, err := net.Listen("tcp", localAddr)
	if local == nil {
		log.Fatalf("Failed to open listening socket: %s", err)
	}
	log.Printf("Starting, listening on %s", localAddr)

	for {
		conn, err := local.Accept()
		if err != nil {
			log.Fatalf("Accept failed: %s", err)
		}
		remote := backends.connect()
		if remote == nil {
			log.Printf("Failed to connect to any backend")
			local.Close()
		} else {
			go forward(conn.(*net.TCPConn), remote)
		}
	}
}
