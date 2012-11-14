// A simple MySQL proxy
//
// Connect to the addresses in order - if one works use it
//
// On any sort of error then close the connection so the client can retry

package main

import (
	"io"
	"log"
	"net"
	"os"
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
func forward(local *net.TCPConn, remoteAddrs []string) {
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
	log.Printf("Start transfer %s to %s", local.LocalAddr(), remote.LocalAddr())
	go copy_half(local, remote, &wg)
	go copy_half(remote, local, &wg)
	wg.Wait()
	log.Printf("Finished transfer from %s to %s done", local.LocalAddr(), remote.LocalAddr())
}

// Main script
func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Syntax: %s <local-address> [<remote-address>]+", os.Args[0])
	}
	localAddr := os.Args[1]
	remoteAddrs := os.Args[2:]
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
		go forward(conn.(*net.TCPConn), remoteAddrs)
	}
}
