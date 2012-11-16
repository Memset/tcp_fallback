// This tests the tcp_fallback package
package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"
)

const (
	proxy   = "127.0.0.1:18080"
	server1 = "127.0.0.1:18081"
	server2 = "127.0.0.1:18082"
	server3 = "127.0.0.1:18083"
)

//Echo server struct
type EchoServer struct {
	address string
	listen  net.Listener
	done    chan bool
	closing bool
}

// Respond to incoming connection
//
// Write the address connected to then echo
func (es *EchoServer) respond(remote *net.TCPConn) {
	defer remote.Close()
	fmt.Fprintf(remote, "%s\n", es.address)
	_, err := io.Copy(remote, remote)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Listen for incoming connections
func (es *EchoServer) serve() {
	for {
		conn, err := es.listen.Accept()
		if es.closing {
			if err == nil {
				conn.Close()
			}
			break
		}
		if err != nil {
			log.Printf("Accept failed: %v", err)
			break
		}
		go es.respond(conn.(*net.TCPConn))
	}
	es.done <- true
}

// Stop the server by closing the listening listen
func (es *EchoServer) stop() {
	es.closing = true
	es.listen.Close()
	<-es.done
}

func NewEchoServer(address string) *EchoServer {
	es := &EchoServer{
		address: address,
		done:    make(chan bool),
	}
	listen, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to open listening socket: %s", err)
	}
	es.listen = listen
	go es.serve()
	return es
}

// Tests the echo server at address to see if it is working
// expects to be connected to remote echo server
func testServer(t *testing.T, address, expected string) {
	remote, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	expected += "\n"
	buf := make([]byte, len(expected))
	io.ReadFull(remote, buf)
	if string(buf) != expected {
		t.Fatal("Failed to read remote server string")
	}
	// Write some random bytes and check they come back
	outbuf := make([]byte, 1024)
	inbuf := make([]byte, 1024)
	for block := 100 + rand.Intn(100); block > 0; block-- {
		blockSize := 500 + rand.Intn(500)
		outbuf = outbuf[:blockSize]
		inbuf = inbuf[:blockSize]
		for i := 0; i < blockSize; i++ {
			outbuf[i] = byte(rand.Intn(256))
		}
		n, err := remote.Write(outbuf)
		if err != nil {
			t.Fatal("Failed to write buffer", err)
		}
		if n != blockSize {
			t.Fatal("Wrote wrong number of bytes", err)
		}
		n, err = remote.Read(inbuf)
		if n != blockSize {
			t.Fatal("Read back wrong number of bytes", err)
		}
		if bytes.Compare(outbuf, inbuf) != 0 {
			t.Fatal("Blocks didn't match")
		}
	}
	err = remote.Close()
	if err != nil {
		t.Fatal("Error on close", err)
	}

}

// Test functions are run in order - this one must be first!
func TestStartServer(t *testing.T) {
	es := NewEchoServer(server1)
	testServer(t, server1, server1)
	es.stop()
}

// Test two servers
func TestServer1(t *testing.T) {
	es1 := NewEchoServer(server1)
	es2 := NewEchoServer(server2)
	testServer(t, server1, server1)
	testServer(t, server2, server2)
	es2.stop()
	es1.stop()
}

// Start the main process
func TestStartMain(t *testing.T) {
	es1 := NewEchoServer(server1)
	os.Args = []string{os.Args[0], "-probe-delay=100ms", proxy, server1, server2, server3}
	go main()
	time.Sleep(1 * time.Second)
	log.SetOutput(ioutil.Discard)
	testServer(t, proxy, server1)
	es1.stop()

}

// Test with a different server
func TestServer2(t *testing.T) {
	es2 := NewEchoServer(server2)
	time.Sleep(1 * time.Second)
	testServer(t, proxy, server2)
	es2.stop()
}

// Test with a different server
func TestServer3(t *testing.T) {
	es3 := NewEchoServer(server3)
	time.Sleep(1 * time.Second)
	testServer(t, proxy, server3)
	es3.stop()
}

// Test with all servers
func TestServerAll(t *testing.T) {
	es1 := NewEchoServer(server1)
	es2 := NewEchoServer(server2)
	es3 := NewEchoServer(server3)
	time.Sleep(1 * time.Second)
	testServer(t, proxy, server1)
	es3.stop()
	time.Sleep(1 * time.Second)
	testServer(t, proxy, server1)
	es1.stop()
	time.Sleep(1 * time.Second)
	testServer(t, proxy, server2)
	es2.stop()
}
