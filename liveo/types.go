package liveo

import (
	"strconv"
	"time"

	"github.com/fivegreenapples/live-o-results/lcs"
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

// DeltaTo produces a ResultDelta as the result of B-A
func (A ResultDataSet) DeltaTo(B ResultDataSet) ResultDelta {

	delta := ResultDelta{
		Old: A.Hash,
		New: B.Hash,
	}

	if A.Results.Title != B.Results.Title {
		delta.Title = &(B.Results.Title)
	}

	coursesA, coursesB := []string{}, []string{}
	coursesBMappings := map[string]Course{} // so we can find new courses added in B
	courseBtoAMappings := map[int]int{}     // so we can compare competitors in common courses
	for i, c := range A.Results.Courses {
		coursesA = append(coursesA, c.Title+"|"+c.Info)
		courseBtoAMappings[i] = i // init map assuming courses don't differ
	}
	for _, c := range B.Results.Courses {
		coursesB = append(coursesB, c.Title+"|"+c.Info)
		coursesBMappings[c.Title+"|"+c.Info] = c
	}
	coursesCommon := lcs.Calculate(coursesA, coursesB)
	if len(coursesCommon) != len(coursesA) || len(coursesCommon) != len(coursesB) {
		// the courses differ
		coursesDelta := CoursesDelta{
			Removed: map[int]int{},
			Added:   map[int]Course{},
		}
		courseBtoAMappings = map[int]int{} // reset these mappings

		cursorA, cursorB, cursorCom := 0, 0, 0
		for cursorCom < len(coursesCommon) || cursorA < len(coursesA) || cursorB < len(coursesB) {
			for cursorCom < len(coursesCommon) &&
				cursorA < len(coursesA) &&
				cursorB < len(coursesB) &&
				coursesCommon[cursorCom] == coursesA[cursorA] &&
				coursesCommon[cursorCom] == coursesB[cursorB] {
				// common item
				// store mapping for competitor comparison
				courseBtoAMappings[cursorB] = cursorA
				// move on all cursors
				cursorCom++
				cursorA++
				cursorB++
			}
			for cursorA < len(coursesA) &&
				(cursorCom >= len(coursesCommon) || coursesCommon[cursorCom] != coursesA[cursorA]) {
				coursesDelta.Removed[cursorA] = 0
				cursorA++
			}
			for cursorB < len(coursesB) &&
				(cursorCom >= len(coursesCommon) || coursesCommon[cursorCom] != coursesB[cursorB]) {
				coursesDelta.Added[cursorB] = coursesBMappings[coursesB[cursorB]]
				cursorB++
			}
		}

		delta.Courses = &coursesDelta
	}

	// Competitor diff analysis. Same as above but for each common group
	// This diff analysis could do with refactor and generics

	for bIndex, aIndex := range courseBtoAMappings {
		courseA := A.Results.Courses[aIndex]
		courseB := B.Results.Courses[bIndex]
		competitorsA, competitorsB := []string{}, []string{}
		competitorsBMappings := map[string]Competitor{} // so we can find new competitors added in B
		for _, c := range courseA.Competitors {
			ident := c.Name + "|" + c.Club + "|" + c.Time.String() + "|" + strconv.FormatBool(c.Valid)
			competitorsA = append(competitorsA, ident)
		}
		for _, c := range courseB.Competitors {
			ident := c.Name + "|" + c.Club + "|" + c.Time.String() + "|" + strconv.FormatBool(c.Valid)
			competitorsB = append(competitorsB, ident)
			competitorsBMappings[ident] = c
		}
		competitorsCommon := lcs.Calculate(competitorsA, competitorsB)
		if len(competitorsCommon) != len(competitorsA) || len(competitorsCommon) != len(competitorsB) {
			// the competitors differ
			competitorsDelta := CompetitorsDelta{
				Removed: map[int]int{},
				Added:   map[int]Competitor{},
			}

			cursorA, cursorB, cursorCom := 0, 0, 0
			for cursorCom < len(competitorsCommon) || cursorA < len(competitorsA) || cursorB < len(competitorsB) {
				for cursorCom < len(competitorsCommon) &&
					cursorA < len(competitorsA) &&
					cursorB < len(competitorsB) &&
					competitorsCommon[cursorCom] == competitorsA[cursorA] &&
					competitorsCommon[cursorCom] == competitorsB[cursorB] {
					// common item
					// move on all cursors
					cursorCom++
					cursorA++
					cursorB++
				}
				for cursorA < len(competitorsA) &&
					(cursorCom >= len(competitorsCommon) || competitorsCommon[cursorCom] != competitorsA[cursorA]) {
					competitorsDelta.Removed[cursorA] = 0
					cursorA++
				}
				for cursorB < len(competitorsB) &&
					(cursorCom >= len(competitorsCommon) || competitorsCommon[cursorCom] != competitorsB[cursorB]) {
					competitorsDelta.Added[cursorB] = competitorsBMappings[competitorsB[cursorB]]
					cursorB++
				}
			}

			if delta.Competitors == nil {
				delta.Competitors = &map[int]CompetitorsDelta{}
			}
			(*delta.Competitors)[bIndex] = competitorsDelta
		}
	}

	return delta
}
func (A ResultDataSet) Clone() ResultDataSet {
	newResultSet := ResultDataSet{}

	newResultSet.Hash = A.Hash
	newResultSet.Results = Results{}

	newResultSet.Results.Title = A.Results.Title
	newResultSet.Results.Courses = make([]Course, 0)

	for _, c := range A.Results.Courses {
		newC := Course{}
		newC.Title = c.Title
		newC.Info = c.Info
		newC.Competitors = make([]Competitor, 0)

		for _, cp := range c.Competitors {
			newCP := Competitor{}
			newCP.Name = cp.Name
			newCP.AgeClass = cp.AgeClass
			newCP.Club = cp.Club
			newCP.Valid = cp.Valid
			newCP.Time = cp.Time

			newC.Competitors = append(newC.Competitors, newCP)
		}

		newResultSet.Results.Courses = append(newResultSet.Results.Courses, newC)
	}

	return newResultSet

}

// ResultDelta encodes the difference between two ResultDataSets
type ResultDelta struct {
	Old         uint64                    `json:"-"`
	New         uint64                    `json:"-"`
	Title       *string                   `json:",omitempty"`
	Courses     *CoursesDelta             `json:",omitempty"`
	Competitors *map[int]CompetitorsDelta `json:",omitempty"`
}

// CoursesDelta encodes the difference between two Courses
type CoursesDelta struct {
	Removed map[int]int
	Added   map[int]Course
}

// CompetitorsDelta encodes the difference between two Competitors
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
	Name     string
	AgeClass string
	Club     string
	Time     time.Duration
	Valid    bool
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
