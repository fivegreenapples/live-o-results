package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/rpc"
	"os"
	"time"

	"github.com/fivegreenapples/live-o-results/liveo"

	"gopkg.in/igm/sockjs-go.v2/sockjs"

	"sync"

	"io/ioutil"
)

func main() {

	listenInterface := flag.String("interface", "", "HTTP Port")
	htdocs := flag.String("htdocs", "", "Docroot of Results Web Site")
	flag.Parse()
	if *listenInterface == "" {
		log.Fatalln("No interface specified (-interface)")
	}
	if *htdocs == "" {
		log.Fatalln("No htdocs provided (-htdocs)")
	}

	var currentResultSet struct {
		sync.RWMutex
		liveo.ResultDataSet
	}
	var resultsWatchers struct {
		sync.RWMutex
		w []func(liveo.ResultDelta)
	}
	rr := newResultsReceiver(func(r liveo.ResultDataSet) {
		currentResultSet.RLock()
		delta := currentResultSet.ResultDataSet.DeltaTo(r)
		currentResultSet.RUnlock()

		currentResultSet.Lock()
		log.Println("Storing new results")
		currentResultSet.ResultDataSet = r
		currentResultSet.Unlock()

		resultsWatchers.RLock()
		for _, watcher := range resultsWatchers.w {
			watcher(delta)
		}
		resultsWatchers.RUnlock()
	})
	rrPublicAPI := ReceiverAPI{rr}

	rpcServer := rpc.NewServer()
	rpcErr := rpcServer.RegisterName("Api", &rrPublicAPI)
	if rpcErr != nil {
		log.Panic(rpcErr)
	}

	http.HandleFunc(liveo.RPCEndpoint, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusMethodNotAllowed)
			io.WriteString(w, "405 must CONNECT\n")
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "500 "+err.Error()+"\n")
			return
		}
		if string(body) != liveo.SharedSecret {
			fmt.Println("Unauthorized " + string(body))
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, "401 Unauthorized\n")
			return
		}

		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "500 "+err.Error()+"\n")
			return
		}
		conn.SetDeadline(time.Time{})

		io.WriteString(conn, "HTTP/1.0 "+liveo.RPCConnectedStatus+"\r\n\r\n")
		log.Printf("Handling RPC Connection from %s", req.RemoteAddr)
		rpcServer.ServeConn(conn)
	})

	http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(*htdocs))))

	type socketEventMsg struct {
		Type string
		Msg  struct {
			Name string
			Data interface{}
		}
	}
	socketsHandler := sockjs.NewHandler("/sockjs", sockjs.DefaultOptions, func(session sockjs.Session) {

		log.Println("Socket session started", session.ID())

		var sendResults = func(res liveo.ResultDataSet) {
			ev := socketEventMsg{}
			ev.Type = "Event"
			ev.Msg.Name = "NewResults"
			ev.Msg.Data = res
			evMsg, _ := json.Marshal(&ev)
			session.Send(string(evMsg))
		}

		// Set up watcher to send deltas for new results
		var sendDelta = func(delta liveo.ResultDelta) {
			ev := socketEventMsg{}
			ev.Type = "Event"
			ev.Msg.Name = "NewDelta"
			ev.Msg.Data = delta
			evMsg, _ := json.Marshal(&ev)
			session.Send(string(evMsg))
		}
		resultsWatchers.Lock()
		resultsWatchers.w = append(resultsWatchers.w, sendDelta)
		resultsWatchers.Unlock()

		// Some handler for incoming messages - this seems pointless
		var sockRecvErr error
		var msg string
		for {
			if msg, sockRecvErr = session.Recv(); sockRecvErr == nil {
				if msg == "RequestResults" {
					currentResultSet.RLock()
					sendResults(currentResultSet.ResultDataSet)
					currentResultSet.RUnlock()
				}
				continue
			}
			break
		}

		log.Println("Socket session ended", session.ID())
	})
	http.Handle("/sockjs/", socketsHandler)

	//
	// Create a server with explicit read and write timeouts
	//
	srv := &http.Server{
		Addr:         *listenInterface,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Launch server and check for errors
	fmt.Println("Launching server")
	err := srv.ListenAndServe()
	if err != nil {
		fmt.Println("Couldn't start server: " + err.Error())
		os.Exit(1)
	}
}
