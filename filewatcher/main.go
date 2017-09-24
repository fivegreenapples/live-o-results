package main

import (
	"log"
)

func main() {
	resultsFile := "/Users/ben/Documents/Orienteering/British Sprints/Live Results/index.html"
	resultsServers := []string{"127.0.0.1:9000"}

	rw := newFileWatcher()
	newFileWatcherManager(rw)

	watchErr := rw.startWatchingFile(resultsFile)
	if watchErr != nil {
		log.Println(watchErr)
	}

	for _, s := range resultsServers {
		rsErr := rw.addResultsServer(s)
		if rsErr != nil {
			log.Println(rsErr)
		}
	}

	rw.wait()

}
