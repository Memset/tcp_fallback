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
)

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
func forward(local *net.TCPConn, remoteAddrs []string, debug bool) {
	var remote *net.TCPConn
	var err error
	for _, remoteAddr := range remoteAddrs {
		remote_conn, err := net.Dial("tcp", remoteAddr)
		if err == nil {
			remote = remote_conn.(*net.TCPConn)
			break
		}
		log.Printf("Failed to connect to remote %s: %s", remoteAddr, err)
	}
	if err != nil {
		log.Printf("Failed to connect to any remotes")
		local.Close()
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	if debug {
		log.Printf("[%s]: Start transfer %s to %s", remote.RemoteAddr(), local.LocalAddr(), remote.LocalAddr())
	}
	go copy_half(local, remote, &wg)
	go copy_half(remote, local, &wg)
	wg.Wait()
	if debug {
		log.Printf("[%s]: Finished transfer from %s to %s done", remote.RemoteAddr(), local.LocalAddr(), remote.LocalAddr())
	}
}

// Main script
func main() {
	me := path.Base(os.Args[0])
	use_syslog := flag.Bool("syslog", false, "Use Syslog for logging")
	debug := flag.Bool("debug", false, "Enable verbose logging")

	flag.Usage = func() {
		fmt.Printf("Usage: %s [flags] <local-address:port> [<remote-address:port>]+\n\n", me)
		fmt.Printf("flags:\n\n")
		flag.PrintDefaults()
		fmt.Printf("\n")
	}
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
	remoteAddrs := flag.Args()[1:]
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
		go forward(conn.(*net.TCPConn), remoteAddrs, *debug)
	}
}
