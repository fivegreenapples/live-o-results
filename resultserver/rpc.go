package main

import (
	"log"

	"github.com/fivegreenapples/live-o-results/liveo"
)

type ReceiverAPI struct {
	rr *resultsReceiver
}

func (a *ReceiverAPI) SubmitLatestResults(r *liveo.ResultDataSet, res *bool) error {
	log.Println("Received results.", r.Hash, r.Results.Title)
	a.rr.submitNewResults(*r)
	*res = true
	return nil
}

func (a *ReceiverAPI) SubmitDelta(r *liveo.ResultDelta, res *bool) error {
	log.Println("Received results delta.", r.Old, r.New)
	err := a.rr.submitDelta(*r)
	if err != nil {
		log.Println("delta error: ", err)
		*res = false
		return nil
	}
	*res = true
	return nil
}
