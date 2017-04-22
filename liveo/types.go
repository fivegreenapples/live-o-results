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
// ResultDelta.Old
type ResultDataSet struct {
	Results Results
	Hash    uint64
}

// ResultDelta encodes the difference between two ResultDataSets
type ResultDelta struct {
	Old         uint64
	New         uint64
	Title       *string
	Courses     *CoursesDelta
	Competitors *map[int]CompetitorsDelta
}
type CoursesDelta struct {
	Removed map[int]int
	Added   map[int]Course
}
type CompetitorsDelta struct {
	Removed map[int]int
	Added   map[int]Competitor
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
	Name  string
	Club  string
	Time  time.Duration
	Valid bool
}

// // Status encodes a Competitor's finish status
// type Status int

// // Status encodes a Competitors finish status
// const (
// 	OK     Status = 0
// 	NoTime        = 1 << iota
// 	Retired
// 	MissPunched
// )
