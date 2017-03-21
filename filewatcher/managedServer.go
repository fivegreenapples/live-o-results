package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/fivegreenapples/live-o-results/liveo"

	"fmt"
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
	conn, err := net.Dial("tcp", m.address)
	if err != nil {
		return err
	}
	httpRequest := "CONNECT " + liveo.RPCEndpoint + " HTTP/1.0\r\n"
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
