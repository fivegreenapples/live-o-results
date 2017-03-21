package main

import (
	"log"

	"github.com/fivegreenapples/live-o-results/liveo"
)

type ReceiverAPI struct {
	rr *resultsReceiver
}

func (a *ReceiverAPI) SubmitLatestResults(r *liveo.ResultDataSet, res *bool) error {
	log.Println("Received results.", r.Results.Title)
	a.rr.submitNewResults(*r)
	*res = true
	return nil
}

func (a *ReceiverAPI) SubmitDelta(r *liveo.ResultDataSet, res *bool) error {
	*res = true
	return nil
}
