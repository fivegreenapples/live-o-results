package main

import (
	"errors"

	"github.com/fivegreenapples/live-o-results/liveo"
	"github.com/mitchellh/hashstructure"
)

type resultsReceiver struct {
	controlCh      chan interface{}
	doneCh         chan struct{}
	resultCallback func(liveo.ResultDataSet)
}

type evNewResultSet struct {
	resultSet liveo.ResultDataSet
	result    chan error
}
type evNewDelta struct {
	delta  liveo.ResultDelta
	result chan error
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
		case evNewDelta:
			if currentResultSet.Hash != ev.delta.Old {
				ev.result <- errors.New("delta from unknown base")
				continue
			}

			// clone current result set
			newResultSet := currentResultSet
			// copy over updated details
			if ev.delta.Title != nil {
				newResultSet.Results.Title = *ev.delta.Title
			}

			if ev.delta.Courses != nil {
				cursorA, cursorB := 0, 0
				toAdd := len(ev.delta.Courses.Added)
				newResultSet.Results.Courses = []liveo.Course{}
				for toAdd > 0 || cursorA < len(currentResultSet.Results.Courses) {
					c, found := ev.delta.Courses.Added[cursorB]
					if found {
						newResultSet.Results.Courses = append(newResultSet.Results.Courses, c)
						toAdd--
						cursorB++
						continue
					}

					_, removed := ev.delta.Courses.Removed[cursorA]
					if !removed {
						newResultSet.Results.Courses = append(newResultSet.Results.Courses, currentResultSet.Results.Courses[cursorA])
					}
					cursorA++
					cursorB++
				}
			}

			if ev.delta.Competitors != nil {

				for courseIndex, compDelta := range *ev.delta.Competitors {
					cursorA, cursorB := 0, 0
					toAdd := len(compDelta.Added)
					oldSet := newResultSet.Results.Courses[courseIndex].Competitors
					newSet := []liveo.Competitor{}
					for toAdd > 0 || cursorA < len(oldSet) {
						c, found := compDelta.Added[cursorB]
						if found {
							newSet = append(newSet, c)
							toAdd--
							cursorB++
							continue
						}

						_, removed := compDelta.Removed[cursorA]
						if !removed {
							newSet = append(newSet, oldSet[cursorA])
						}
						cursorA++
						cursorB++
					}
					newResultSet.Results.Courses[courseIndex].Competitors = newSet
				}
			}

			// check resulting hash
			hash, _ := hashstructure.Hash(newResultSet.Results, nil)
			if hash != ev.delta.New {
				ev.result <- errors.New("patch results didn't match expected hash")
				continue
			}

			// store hash
			newResultSet.Hash = hash
			// copy back
			currentResultSet = newResultSet

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
func (r *resultsReceiver) submitDelta(delta liveo.ResultDelta) error {
	resultCh := make(chan error)
	r.controlCh <- evNewDelta{
		delta:  delta,
		result: resultCh,
	}
	return <-resultCh
}
