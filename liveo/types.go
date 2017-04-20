package liveo

import (
	"time"
)

// SharedSecret is used to ensure communication from to a ResultServer is from a trusted source
var SharedSecret = "a565baf1712cf73eafa88d4ccea182c12ad8f6c1fa0d40184bd52d056082890d"

// RPCConnectedStatus encodes the expected response from an RPC client connect to a ResultServer
// The use of a 101 (switching protocols) response code means we can traverse nginx reverse proxying,
var RPCConnectedStatus = "101 Connected to Otheday"

// RPCEndpoint stores the RPC endpoint URI path
var RPCEndpoint = "/api"

// A ResultDataSet represents the entire set of results as exported from AutoDownload. Hash is used
// for integrity checking following transmission across the internet, and as an identifier for use as
// ResultDelta.Base
type ResultDataSet struct {
	Results Results
	Hash    string
}

// ResultDelta encodes the difference between two ResultDataSets
type ResultDelta struct {
	Base       string
	Title      string
	NewCourses []struct {
		Index  int
		Course Course
	}
	NewCompetitor []struct {
		Course     string
		Index      int
		Competitor Competitor
	}
}

// Results defines a set of orienteering results as published from AutoDownload
type Results struct {
	Title   string
	Courses []Course
}

// Course reperesents course information within a result set.
type Course struct {
	Title       string
	Info        string
	Competitors []Competitor
}

// Competitor represents a particular runner and their time and position within a set
// of results.
type Competitor struct {
	Name   string
	Club   string
	Time   time.Duration
	Status Status
}

// Status encodes a Competitor's finish status
type Status int

// Status encodes a Competitors finish status
const (
	OK     Status = 0
	NoTime        = 1 << iota
	Retired
	MissPunched
)
