package main

import "github.com/fivegreenapples/live-o-results/liveo"

type resultsReceiver struct {
	controlCh      chan interface{}
	doneCh         chan struct{}
	resultCallback func(liveo.ResultDataSet)
}

type evNewResultSet struct {
	resultSet liveo.ResultDataSet
	result    chan error
}
type evStop struct{}

func newResultsReceiver(cb func(liveo.ResultDataSet)) *resultsReceiver {
	r := resultsReceiver{
		controlCh:      make(chan interface{}),
		doneCh:         make(chan struct{}),
		resultCallback: cb,
	}
	go r.run()
	return &r
}

func (r *resultsReceiver) run() {

	var currentResultSet liveo.ResultDataSet

RANGELOOP:
	for ev := range r.controlCh {
		switch ev := ev.(type) {
		case evNewResultSet:
			currentResultSet = ev.resultSet
			r.resultCallback(currentResultSet)
			ev.result <- nil
		case evStop:
			break RANGELOOP
		}
	}
	r.doneCh <- struct{}{}
}

func (r *resultsReceiver) wait() {
	<-r.doneCh
}
func (r *resultsReceiver) stop() {
	r.controlCh <- evStop{}
}
func (r *resultsReceiver) submitNewResults(set liveo.ResultDataSet) {
	resultCh := make(chan error)
	r.controlCh <- evNewResultSet{
		resultSet: set,
		result:    resultCh,
	}
	<-resultCh
}
