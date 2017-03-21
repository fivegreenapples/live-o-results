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
	"strconv"
	"time"

	"github.com/fivegreenapples/live-o-results/liveo"

	"gopkg.in/igm/sockjs-go.v2/sockjs"

	"sync"

	"io/ioutil"
)

func main() {

	httpPort := flag.Uint("port", 0, "HTTP Port")
	htdocs := flag.String("htdocs", "", "Docroot of Results Web Site")
	flag.Parse()
	if *httpPort == 0 {
		log.Fatalln("No HTTP port specified (-port)")
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
		w []func(liveo.ResultDataSet)
	}
	rr := newResultsReceiver(func(r liveo.ResultDataSet) {
		log.Println("Storing new results")
		currentResultSet.Lock()
		currentResultSet.ResultDataSet = r
		currentResultSet.Unlock()
		currentResultSet.RLock()
		resultsWatchers.RLock()
		for _, watcher := range resultsWatchers.w {
			watcher(currentResultSet.ResultDataSet)
		}
		resultsWatchers.RUnlock()
		currentResultSet.RUnlock()
	})
	rrPublicAPI := ReceiverAPI{rr}

	rpcServer := rpc.NewServer()
	rpcErr := rpcServer.RegisterName("Api", &rrPublicAPI)
	if rpcErr != nil {
		log.Panic(rpcErr)
	}

	http.HandleFunc(liveo.RPCEndpoint, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "CONNECT" {
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

	http.Handle("/results/", http.StripPrefix("/results/", http.FileServer(http.Dir(*htdocs))))

	socketsHandler := sockjs.NewHandler("/sockjs", sockjs.DefaultOptions, func(session sockjs.Session) {
		for {
			var sendNewResults = func(r liveo.ResultDataSet) {
				currentResultsMsg, _ := json.Marshal(&struct {
					Type string
					Msg  interface{}
				}{
					Type: "Event",
					Msg: struct {
						Name string
						Data interface{}
					}{"newResults", currentResultSet.ResultDataSet},
				})
				session.Send(string(currentResultsMsg))
			}
			currentResultSet.RLock()
			copyCurrentResultSet := currentResultSet.ResultDataSet
			currentResultSet.RUnlock()
			sendNewResults(copyCurrentResultSet)

			resultsWatchers.Lock()
			resultsWatchers.w = append(resultsWatchers.w, sendNewResults)
			resultsWatchers.Unlock()

			if msg, err := session.Recv(); err == nil {
				session.Send(msg + msg)
			}
			break
		}
	})
	http.Handle("/sockjs/", socketsHandler)

	//
	// Create a server with explicit read and write timeouts
	//
	srv := &http.Server{
		Addr:         "localhost:" + strconv.Itoa(int(*httpPort)),
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
