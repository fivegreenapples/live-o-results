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
	"time"
)

type managedServer struct {
	address       string
	rpcClient     *rpc.Client
	lastResultset *liveo.ResultDataSet
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
	dialTimeout := 5 * time.Second
	if port == "443" {
		dialer := net.Dialer{Timeout: dialTimeout}
		conn, err = tls.DialWithDialer(&dialer, "tcp", net.JoinHostPort(host, port), nil)
	} else {
		conn, err = net.DialTimeout("tcp", net.JoinHostPort(host, port), dialTimeout)
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
		err = errors.New("managedServer: unexpected HTTP response: " + resp.Status)
		return err
	}

	// Reset deadline so we don't lose the connection
	conn.SetDeadline(time.Time{})

	m.rpcClient = rpc.NewClient(conn)
	return nil
}

func (m *managedServer) submitResults(rs liveo.ResultDataSet) {

	// todo: prevent overlappping calls.

	method := "Api.SubmitLatestResults"
	var args interface{} = &rs

	if m.lastResultset != nil {
		delta := m.lastResultset.DeltaTo(rs)
		method = "Api.SubmitDelta"
		args = &delta
	}

	var network bytes.Buffer        // Stand-in for a network connection
	enc := gob.NewEncoder(&network) // Will write to network.
	enc.Encode(args)
	log.Printf("managedServer: gob encoding of data set is %d bytes.", len(network.Bytes()))

	if m.rpcClient == nil {
		dialErr := m.dial()
		if dialErr != nil {
			log.Println("managedServer: failed to dial results server:", dialErr)
			return
		}
		log.Println("managedServer: successfully re-dialed when making rpc call")
	}

	var reply bool
	call := m.rpcClient.Go(method, args, &reply, nil)
	go func() {
		select {
		case <-call.Done:
			// ok
		case <-time.After(5 * time.Second):
			// timed out. close the connection...
			log.Println("managedServer: rpc call timed out")
			closeErr := m.rpcClient.Close()
			if closeErr != nil {
				log.Println("managedServer: error closing connection after rpc call timed out:", closeErr)
				m.rpcClient = nil
				return
			}
			// ...and wait for completion.
			// this shouldn't block as we just killed the connection.
			<-call.Done
		}

		if call.Error != nil {
			log.Println("managedServer: rpc call error:", call.Error)
			m.rpcClient = nil
			// possibly we should retry submitting results here but we probably need some
			// extra work to avoid getting stuck in a loop e.g. where dialling succeeds but
			// the rpc call fails. For whatever weird reason.
			return
		}

		if reply {
			// record this result set as the last successful
			m.lastResultset = &rs
		} else {
			// only get here on a failed delta submission. reset lastresultset
			m.lastResultset = nil
		}

	}()
}
