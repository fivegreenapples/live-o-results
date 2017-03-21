package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"gopkg.in/igm/sockjs-go.v2/sockjs"

	"encoding/json"

	"io/ioutil"

	"fmt"
)

type fileWatcherManager struct {
	rw       *fileWatcher
	demuxMap map[string]rwmHandler
}

func newFileWatcherManager(rw *fileWatcher) *fileWatcherManager {
	rwm := fileWatcherManager{
		rw:       rw,
		demuxMap: map[string]rwmHandler{},
	}
	rwm.demuxMap["status.get"] = rwm.doGetStatus
	rwm.demuxMap["resultsserver.add"] = rwmHandler(rwm.doAddResultsServer).withRequestType(reflect.TypeOf(AddResultsServerRequest{}))
	rwm.demuxMap["resultsserver.remove"] = rwmHandler(rwm.doRemoveResultsServer).withRequestType(reflect.TypeOf(RemoveResultsServerRequest{}))
	rwm.demuxMap["filewatch.start"] = rwmHandler(rwm.doStartFileWatch).withRequestType(reflect.TypeOf(StartFilewatchRequest{}))
	rwm.demuxMap["filewatch.stop"] = rwm.doStopFileWatch

	http.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir("/Users/ben/Documents/Coding/otheday-ui/watcher-manager"))))

	http.HandleFunc("/status", rwm.demuxMap["status.get"].AllowGet())
	http.HandleFunc("/filewatch/start", rwm.demuxMap["filewatch.start"].AllowPost())
	http.HandleFunc("/filewatch/stop", rwm.demuxMap["filewatch.stop"].AllowPost())
	http.HandleFunc("/resultsserver/add", rwm.demuxMap["resultsserver.add"].AllowPost())
	http.HandleFunc("/resultsserver/remove", rwm.demuxMap["resultsserver.remove"].AllowPost())

	socketsHandler := sockjs.NewHandler("/sockjs", sockjs.DefaultOptions, func(session sockjs.Session) {

		rwm.rw.registerStatusListener(func(s fileWatcherStatus) {
			statusMsg, _ := json.Marshal(&socketMessageOut{
				Type: "Event",
				Msg: struct {
					Name string
					Data interface{}
				}{"statuschanged", s},
			})
			session.Send(string(statusMsg))
		})

		for {
			if msg, err := session.Recv(); err == nil {

				var m socketMessageIn
				jsonErr := json.Unmarshal([]byte(msg), &m)
				if jsonErr != nil {
					session.Send(`{"Type":"Error","Msg":"Failed json unmarshall: ` + jsonErr.Error() + `"}`)
				}

				if m.Type == "API" {
					apiMsg := struct {
						Tag int
						Req json.RawMessage
					}{}
					jsonErr := json.Unmarshal(m.Msg, &apiMsg)
					if jsonErr != nil {
						session.Send(`{"Type":"Error","Msg":"Failed json unmarshall of API request: ` + jsonErr.Error() + `"}`)
					}

					response := rwm.demuxAPIRequest(apiMsg.Req)
					apiResponseMsg, _ := json.Marshal(&socketMessageOut{
						Type: "API",
						Msg: &struct {
							Tag  int
							Resp interface{}
						}{apiMsg.Tag, (*json.RawMessage)(&response)},
					})
					session.Send(string(apiResponseMsg))
				} else {
					session.Send(`{"Type":"Error","Msg":"Unhandled message type"}`)
				}

				continue
			}
			break
		}
	})
	http.Handle("/sockjs/", socketsHandler)

	//
	// Create a server with explicit read and write timeouts
	//
	srv := &http.Server{
		Addr:         "localhost:8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	// Launch server and check for errors
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Panicln("Couldn't start manager: ", err)
			os.Exit(1)
		}
	}()

	return &rwm
}
func (rwm *fileWatcherManager) demuxAPIRequest(req []byte) []byte {
	var baseReq baseRequest
	jsonErr := json.Unmarshal(req, &baseReq)
	if jsonErr != nil {
		return response(false, "JSON request decode error: "+jsonErr.Error(), nil)
	}

	if baseReq.Action == "" {
		return response(false, "No action supplied", nil)
	}

	actionHandler, found := rwm.demuxMap[baseReq.Action]
	if !found {
		return response(false, fmt.Sprintf("Action not recognised: '%s'", baseReq.Action), nil)
	}

	resp, handlerErr := actionHandler(baseReq.Params)
	if handlerErr != nil {
		return response(false, handlerErr.Error(), nil)
	}
	return response(true, "", resp)
}
func doHandle(method string, handler rwmHandler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-type", "application/json")

		requestBody, bodyErr := ioutil.ReadAll(r.Body)
		if bodyErr != nil {
			w.Write(response(false, "Request body read error: "+bodyErr.Error(), nil))
			return
		}

		resp, handlerErr := handler(json.RawMessage(requestBody))
		if handlerErr != nil {
			w.Write(response(false, handlerErr.Error(), nil))
			return
		}
		w.Write(response(true, "", resp))
	}
}

type rwmHandler func(interface{}) (interface{}, error)

func (h rwmHandler) withRequestType(T reflect.Type) rwmHandler {
	if T == nil {
		panic("withRequestType must be called with a valid reflect.Type")
	}
	return func(params interface{}) (interface{}, error) {
		p := params.(json.RawMessage)

		if len(p) == 0 {
			return nil, errors.New("No request data supplied")
		}

		// Store in reqObj a pointer to a zero value of our request type
		reqObj := reflect.New(T).Interface()
		// Use above to unmarshall into (i.e. unmarshall into the zero value)
		jsonErr := json.Unmarshal(p, reqObj)
		if jsonErr != nil {
			return nil, errors.New("JSON request unmarshall error: " + jsonErr.Error())
		}
		// Reassign into reqObj the actual value (i.e. dereference)
		// We don't need to do this but it means we pass the value in
		// the handler rather than a ptr-to-value. This feels nicer.
		reqObj = reflect.ValueOf(reqObj).Elem().Interface()

		// run the original handler with the unmarshalled object
		result, err := h(reqObj)
		return result, err
	}
}
func (h rwmHandler) AllowPost() http.HandlerFunc {
	return doHandle(http.MethodPost, h)
}
func (h rwmHandler) AllowGet() http.HandlerFunc {
	return doHandle(http.MethodGet, h)
}

type socketMessageOut struct {
	Type string
	Msg  interface{}
}
type socketMessageIn struct {
	Type string
	Msg  json.RawMessage
}
type baseRequest struct {
	Action string
	Params json.RawMessage
}
type StartFilewatchRequest struct {
	File string
}
type AddResultsServerRequest struct {
	ServerAddress string
}
type RemoveResultsServerRequest struct {
	ServerAddress string
}
type genericResponse struct {
	Success      bool
	ErrorMessage string
	Result       interface{}
}

func (rwm *fileWatcherManager) doGetStatus(interface{}) (interface{}, error) {
	return rwm.rw.getStatus(), nil
}
func (rwm *fileWatcherManager) doStartFileWatch(req interface{}) (interface{}, error) {
	request := req.(StartFilewatchRequest)
	err := rwm.rw.startWatchingFile(request.File)
	return nil, err
}
func (rwm *fileWatcherManager) doStopFileWatch(interface{}) (interface{}, error) {
	rwm.rw.stopWatchingFile()
	return nil, nil
}
func (rwm *fileWatcherManager) doAddResultsServer(req interface{}) (interface{}, error) {
	request := req.(AddResultsServerRequest)
	err := rwm.rw.addResultsServer(request.ServerAddress)
	return nil, err
}
func (rwm *fileWatcherManager) doRemoveResultsServer(req interface{}) (interface{}, error) {
	request := req.(RemoveResultsServerRequest)
	rwm.rw.removeResultsServer(request.ServerAddress)
	return nil, nil
}

func response(success bool, errorMsg string, result interface{}) []byte {
	resp, jsonErr := json.Marshal(genericResponse{
		Success:      success,
		ErrorMessage: errorMsg,
		Result:       result,
	})
	if jsonErr != nil {
		resp, _ = json.Marshal(genericResponse{
			Success:      false,
			ErrorMessage: jsonErr.Error(),
			Result:       nil,
		})
	}
	return resp
}
