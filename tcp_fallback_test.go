// This tests the tcp_fallback package
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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	proxy   = "127.0.0.1:18080"
	server1 = "127.0.0.1:18081"
	server2 = "127.0.0.1:18082"
	server3 = "127.0.0.1:18083"
)

var (
	tmpLogFile, _ = ioutil.TempFile("", "tcp_fallback-test-")
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
func testServer(t *testing.T, address, expected string) int {
	remote, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	expected += "\n"
	buf := make([]byte, len(expected))
	transferred, _ := io.ReadFull(remote, buf)
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
		transferred += n
		n, err = remote.Read(inbuf)
		if n != blockSize {
			t.Fatal("Read back wrong number of bytes", err)
		}
		transferred += n
		if bytes.Compare(outbuf, inbuf) != 0 {
			t.Fatal("Blocks didn't match")
		}
	}
	err = remote.Close()
	if err != nil {
		t.Fatal("Error on close", err)
	}
	return transferred
}

// Find a string in the logfile
func checkLogLine(text string, params ...interface{}) bool {
	line := fmt.Sprintf(text, params...)
	content, err := ioutil.ReadFile(tmpLogFile.Name())
	if err != nil {
		return false
	}
	return strings.Contains(string(content), line)
}

// Delete logfile and send SIGHUP to currrent process to re-open it
// this is used by several log-related tests
func reopenLogFile(t *testing.T) {
	os.Remove(tmpLogFile.Name())
	_, err := os.Stat(tmpLogFile.Name())
	if err == nil {
		t.Fatal("Failed to remove logfile")
	}

	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGHUP)
	time.Sleep(1 * time.Second)
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
	os.Args = []string{os.Args[0], "-logfile", tmpLogFile.Name(), "-probe-delay=100ms", proxy, server1, server2, server3}
	go main()
	time.Sleep(1 * time.Second)
	transferred := testServer(t, proxy, server1)
	es1.stop()

	// Wait the backend stats to be updated
	time.Sleep(1 * time.Second)

	if backends[0].failed == true {
		t.Fatal("server1 failed")
	}
	if backends[0].requests != 1 {
		t.Fatal("1 request expected")
	}
	if backends[0].errors != 0 {
		t.Fatal("no errors expected")
	}
	if backends[0].transferred != int64(transferred) {
		t.Fatal("transferred bytes", transferred, backends[0].transferred)
	}
}

// Test with a different server
func TestServer2(t *testing.T) {
	es2 := NewEchoServer(server2)
	time.Sleep(1 * time.Second)
	transferred := testServer(t, proxy, server2)
	es2.stop()

	// Wait the backend stats to be updated
	time.Sleep(1 * time.Second)

	if backends[0].failed == false {
		t.Fatal("server1 didn't fail")
	}
	if backends[1].failed == true {
		t.Fatal("server2 failed")
	}
	if backends[1].requests != 1 {
		t.Fatal("1 request expected")
	}
	if backends[1].transferred != int64(transferred) {
		t.Fatal("transferred bytes", transferred, backends[1].transferred)
	}
}

// Test with a different server
func TestServer3(t *testing.T) {
	es3 := NewEchoServer(server3)
	time.Sleep(1 * time.Second)
	transferred := testServer(t, proxy, server3)
	es3.stop()

	// Wait the backend stats to be updated
	time.Sleep(1 * time.Second)

	if backends[0].failed == false {
		t.Fatal("server1 didn't fail")
	}
	if backends[1].failed == false {
		t.Fatal("server2 didn't fail")
	}
	if backends[2].failed == true {
		t.Fatal("server3 failed")
	}
	if backends[2].requests != 1 {
		t.Fatal("1 request expected")
	}
	if backends[2].transferred != int64(transferred) {
		t.Fatal("transferred bytes", transferred, backends[1].transferred)
	}
}

// Test with all servers
func TestServerAll(t *testing.T) {
	es1 := NewEchoServer(server1)
	es2 := NewEchoServer(server2)
	es3 := NewEchoServer(server3)
	time.Sleep(1 * time.Second)
	testServer(t, proxy, server1)
	es1.stop()

	if backends[0].failed == true {
		t.Fatal("server1 failed")
	}

	time.Sleep(1 * time.Second)
	testServer(t, proxy, server2)
	es2.stop()

	if backends[0].failed == false {
		t.Fatal("server1 didn't fail")
	}
	if backends[1].failed == true {
		t.Fatal("server2 failed")
	}

	time.Sleep(1 * time.Second)
	testServer(t, proxy, server3)
	es3.stop()

	if backends[0].failed == false {
		t.Fatal("server1 didn't fail")
	}
	if backends[1].failed == false {
		t.Fatal("server2 didn't fail")
	}
	if backends[2].failed == true {
		t.Fatal("server3 failed")
	}

}

// Logfile must be re-opened after SIGHUP
func TestServerHupLogFile(t *testing.T) {
	reopenLogFile(t)

	_, err := os.Stat(tmpLogFile.Name())
	if err != nil {
		t.Fatal("Logfile not re-opened after SIGHUP", err)
	}
	if checkLogLine("SIGHUP received") == false {
		t.Fatal("SIGHUP not aknowledged in logfile")
	}
}

// Log line when backend is back
func TestServerBackendBack(t *testing.T) {
	reopenLogFile(t)

	// Force all backends to be in failed state
	remote, _ := net.Dial("tcp", proxy)
	time.Sleep(1 * time.Second)
	remote.Close()

	if backends[0].failed == false || backends[1].failed == false || backends[2].failed == false {
		t.Fatal("Not all backends in failed state")
	}

	if checkLogLine("Failed to connect to any backend") == false {
		t.Fatal("log line not found")
	}

	es1 := NewEchoServer(server1)
	time.Sleep(1 * time.Second)

	testServer(t, proxy, server1)
	es1.stop()

	if checkLogLine("Backend is back %s", server1) == false {
		t.Fatal("log line not found")
	}

	es2 := NewEchoServer(server2)
	time.Sleep(1 * time.Second)

	testServer(t, proxy, server2)
	es2.stop()

	if checkLogLine("Backend is back %s", server2) == false {
		t.Fatal("log line not found")
	}

	es3 := NewEchoServer(server3)
	time.Sleep(1 * time.Second)

	testServer(t, proxy, server3)
	es3.stop()

	if checkLogLine("Backend is back %s", server3) == false {
		t.Fatal("log line not found")
	}
}

// This must be last test to be run
func TestServerCleanUp(t *testing.T) {
	os.Remove(tmpLogFile.Name())
	tmpLogFile.Close()
}
