package main

import (
	"io/ioutil"
	"reflect"
	"time"

	"log"

	"strings"

	"sync"

	"errors"

	"github.com/fivegreenapples/live-o-results/liveo"
	"github.com/fivegreenapples/throttledwatcher"
)

type fileWatcher struct {
	controlCh chan interface{}
	doneCh    chan struct{}
}
type fileWatcherStatus struct {
	File        string
	ActiveWatch bool
	Servers     []string
}
type evGetStatus struct {
	s chan fileWatcherStatus
}
type evRegisterStatusListener struct {
	l      func(fileWatcherStatus)
	result chan error
}
type evAddServer struct {
	addr   string
	result chan error
}
type evDropServer struct {
	addr   string
	result chan error
}

type evStartFileWatch struct {
	file      string
	quietTime time.Duration
	result    chan error
}
type evStopFileWatch struct {
	result chan error
}
type evStop struct{}

func newFileWatcher() *fileWatcher {
	r := fileWatcher{
		controlCh: make(chan interface{}),
		doneCh:    make(chan struct{}),
	}
	go r.run()
	return &r
}

func (r *fileWatcher) run() {

	allServers := map[string]*managedServer{}
	var watchedFile string
	var fwStopper func()
	var currentResultSet liveo.ResultDataSet

	statusUpdates := make(chan fileWatcherStatus, 10)
	statusListeners := []func(fileWatcherStatus){}
	statusListenersMu := sync.RWMutex{}

	go func() {
		for s := range statusUpdates {
			statusListenersMu.RLock()
			for _, l := range statusListeners {
				l(s)
			}
			statusListenersMu.RUnlock()
		}
	}()

	currentStatus := func() fileWatcherStatus {
		s := fileWatcherStatus{
			File:        watchedFile,
			ActiveWatch: fwStopper != nil,
			Servers:     make([]string, 0, len(allServers)),
		}
		for srv := range allServers {
			s.Servers = append(s.Servers, srv)
		}
		return s
	}

RANGELOOP:
	for ev := range r.controlCh {
		switch ev := ev.(type) {
		case evGetStatus:
			ev.s <- currentStatus()
		case evRegisterStatusListener:
			statusListenersMu.Lock()
			statusListeners = append(statusListeners, ev.l)
			statusListenersMu.Unlock()
			ev.result <- nil
		case evStartFileWatch:
			if fwStopper != nil {
				fwStopper()
				fwStopper = nil
			}
			var err error
			fwStopper, err = startFileWatching(ev.file, ev.quietTime, func(f string) {
				log.Println("watch event for", ev.file)
				newResults, decodeErr := decodeResultsFile(strings.NewReader(f))
				if decodeErr != nil {
					return
				}
				newResultSet := liveo.ResultDataSet{
					Results: *newResults,
					Hash:    "LARK",
				}
				if reflect.DeepEqual(currentResultSet, newResultSet) {
					// ignore results, file hasn't changed
					log.Println("no change, ignoring")
					return
				}
				currentResultSet = newResultSet
				for _, s := range allServers {
					s.submitResults(currentResultSet)
				}
			})
			watchedFile = ev.file
			statusUpdates <- currentStatus()
			if err != nil {
				ev.result <- err
			}
			ev.result <- nil
		case evStopFileWatch:
			if fwStopper != nil {
				fwStopper()
				fwStopper = nil
			}
			statusUpdates <- currentStatus()
			ev.result <- nil
		case evAddServer:
			_, found := allServers[ev.addr]
			if found {
				ev.result <- errors.New("result server is already subscribed")
				continue
			}
			s := managedServer{
				address: ev.addr,
			}
			if err := s.dial(); err != nil {
				ev.result <- err
				continue
			}
			s.submitResults(currentResultSet)
			allServers[ev.addr] = &s
			statusUpdates <- currentStatus()
			ev.result <- nil
		case evDropServer:
			delete(allServers, ev.addr)
			statusUpdates <- currentStatus()
			ev.result <- nil
		case evStop:
			close(statusUpdates)
			break RANGELOOP
		}
	}
	r.doneCh <- struct{}{}
}

func (r *fileWatcher) wait() {
	<-r.doneCh
}
func (r *fileWatcher) stop() {
	r.controlCh <- evStop{}
}
func (r *fileWatcher) getStatus() fileWatcherStatus {
	sCh := make(chan fileWatcherStatus)
	r.controlCh <- evGetStatus{
		s: sCh,
	}
	return <-sCh
}
func (r *fileWatcher) registerStatusListener(l func(fileWatcherStatus)) {
	resultCh := make(chan error)
	r.controlCh <- evRegisterStatusListener{
		l:      l,
		result: resultCh,
	}
	<-resultCh
}
func (r *fileWatcher) addResultsServer(address string) error {
	resultCh := make(chan error)
	r.controlCh <- evAddServer{
		addr:   address,
		result: resultCh,
	}
	return <-resultCh
}
func (r *fileWatcher) removeResultsServer(address string) {
	resultCh := make(chan error)
	r.controlCh <- evDropServer{
		addr:   address,
		result: resultCh,
	}
	<-resultCh
}
func (r *fileWatcher) startWatchingFile(file string) error {
	resultCh := make(chan error)
	r.controlCh <- evStartFileWatch{
		file:      file,
		quietTime: 3 * time.Second,
		result:    resultCh,
	}
	return <-resultCh
}
func (r *fileWatcher) stopWatchingFile() {
	resultCh := make(chan error)
	r.controlCh <- evStopFileWatch{
		result: resultCh,
	}
	<-resultCh
}

func startFileWatching(file string, quietTime time.Duration, cb func(string)) (func(), error) {
	log.Println("Starting watch on ", file)
	watcher, err := throttledwatcher.NewWatcher(file, quietTime)
	if err != nil {
		return nil, err
	}
	stopCh := make(chan struct{})
	active := true
	go func() {
		for {
			select {
			case <-watcher.C:
				fileContents, err := ioutil.ReadFile(file)
				if err != nil {
					// ??
					continue
				}
				cb(string(fileContents))
			case <-stopCh:
				watcher.Stop()
				active = false
				return
			}
		}
	}()
	return func() {
		if active {
			log.Println("Stopping watch on ", file)
			stopCh <- struct{}{}
		}
	}, nil
}
