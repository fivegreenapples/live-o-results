package main

import (
	"log"
)

func main() {
	resultsFile := "/Users/ben/Documents/Orienteering/British Sprints/Live Results/index.html"
	resultsServer := "test.o-results.live"

	rw := newFileWatcher()
	newFileWatcherManager(rw)

	watchErr := rw.startWatchingFile(resultsFile)
	if watchErr != nil {
		log.Println(watchErr)
	}

	rsErr := rw.addResultsServer(resultsServer)
	if rsErr != nil {
		log.Println(rsErr)
	}

	rw.wait()

}
