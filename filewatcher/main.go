package main

import (
	"log"
)

func main() {
	resultsFile := "/Users/ben/Documents/Orienteering/British Sprints/Live Results/index.html"

	rw := newFileWatcher()
	newFileWatcherManager(rw)

	watchErr := rw.startWatchingFile(resultsFile)
	if watchErr != nil {
		log.Println(watchErr)
	}
	rw.wait()

}
