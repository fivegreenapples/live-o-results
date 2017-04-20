package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/fivegreenapples/live-o-results/liveo"

	"fmt"
	"strings"
)

type managedServer struct {
	address   string
	rpcClient *rpc.Client
}

/*
CONNECT /api HTTP/1.0
Content-Length: 64

a565baf1712cf73eafa88d4ccea182c12ad8f6c1fa0d40184bd52d056082890d
*/

func (m *managedServer) dial() error {
	var err error
	var host, port string

	lastColonPos := strings.LastIndex(m.address, ":")
	if lastColonPos == -1 {
		host = m.address
		port = "80"
	} else {
		host, port, err = net.SplitHostPort(m.address)
		if err != nil {
			return err
		}
	}

	var conn net.Conn
	if port == "443" {
		conn, err = tls.Dial("tcp", net.JoinHostPort(host, port), nil)
	} else {
		conn, err = net.Dial("tcp", net.JoinHostPort(host, port))
	}
	if err != nil {
		return err
	}

	httpRequest := "GET " + liveo.RPCEndpoint + " HTTP/1.0\r\n"
	httpRequest += fmt.Sprintf("Host: %s\r\n", host)
	httpRequest += fmt.Sprint("Connection: Upgrade\r\n")
	httpRequest += fmt.Sprint("Upgrade: RPC\r\n")
	httpRequest += fmt.Sprintf("Content-Length: %d\r\n", len(liveo.SharedSecret))
	httpRequest += "\r\n"
	httpRequest += liveo.SharedSecret
	io.WriteString(conn, httpRequest)

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil {
		conn.Close()
		return err
	}
	if resp.Status != liveo.RPCConnectedStatus {
		conn.Close()
		err = errors.New("unexpected HTTP response: " + resp.Status)
		return err
	}

	m.rpcClient = rpc.NewClient(conn)
	return nil
}

func (m *managedServer) submitResults(rs liveo.ResultDataSet) {

	var network bytes.Buffer        // Stand-in for a network connection
	enc := gob.NewEncoder(&network) // Will write to network.
	enc.Encode(rs)
	log.Printf("gob encoding of result set is %d bytes.", len(network.Bytes()))

	var reply bool
	err := m.rpcClient.Call("Api.SubmitLatestResults", &rs, &reply)
	if err != nil {
		log.Println("managedServer rpc call error:", err)
	}
}
