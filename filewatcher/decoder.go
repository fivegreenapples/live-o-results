package main

import (
	"errors"
	"io"
	"sort"
	"strconv"

	"github.com/fivegreenapples/live-o-results/liveo"

	"sync"

	"regexp"
	"time"

	xmlpath "gopkg.in/xmlpath.v2"
)

var decodeOnce sync.Once
var xpTitle *xmlpath.Path
var xpCourse *xmlpath.Path

func decodeResultsFile(file io.Reader) (*liveo.Results, error) {
	decodeOnce.Do(initDecode)

	r := liveo.Results{}

	rootNode, err := xmlpath.ParseHTML(file)
	if err != nil {
		return nil, err
	}

	if value, ok := xpTitle.String(rootNode); ok {
		r.Title = value
		// Trim off preamble
		reg := regexp.MustCompile("^Results for ")
		r.Title = reg.ReplaceAllString(r.Title, "")
		// Trim off postamble
		reg = regexp.MustCompile(" - [0-9]{1,2}-[a-zA-Z]{3}-20[0-9]{2}$")
		r.Title = reg.ReplaceAllString(r.Title, "")
	}

	courses := xpCourse.Iter(rootNode)
	for courses.Next() {
		course, err := decodeCourse(courses.Node())
		if err != nil {
			return nil, err
		}
		r.Courses = append(r.Courses, *course)
	}

	return &r, nil
}

var xpCourseTitle *xmlpath.Path
var xpCourseInfo *xmlpath.Path
var xpCourseCompetitors *xmlpath.Path

func decodeCourse(cn *xmlpath.Node) (*liveo.Course, error) {

	c := liveo.Course{}

	if value, ok := xpCourseTitle.String(cn); ok {
		c.Title = value
	}
	if value, ok := xpCourseInfo.String(cn); ok {
		c.Info = value
	}
	competitors := xpCourseCompetitors.Iter(cn)
	for competitors.Next() {
		competitor, err := decodeCompetitor(competitors.Node())
		if err != nil {
			return nil, err
		}
		c.Competitors = append(c.Competitors, *competitor)
	}
	sort.Slice(c.Competitors, func(i, j int) bool {
		if (c.Competitors[i].Valid && c.Competitors[j].Valid) ||
			(!c.Competitors[i].Valid && !c.Competitors[j].Valid) {
			return c.Competitors[i].Time < c.Competitors[j].Time
		}
		if c.Competitors[i].Valid {
			return true
		}
		return false
	})
	return &c, nil
}

var reValidPosition *regexp.Regexp
var xpCompetitorPosition *xmlpath.Path
var xpCompetitorName *xmlpath.Path
var xpCompetitorClub *xmlpath.Path
var xpCompetitorAgeClass *xmlpath.Path
var xpCompetitorTime *xmlpath.Path

func decodeCompetitor(cn *xmlpath.Node) (*liveo.Competitor, error) {

	c := liveo.Competitor{}

	if value, ok := xpCompetitorPosition.String(cn); ok {
		c.Valid = reValidPosition.MatchString(value)
	}
	if value, ok := xpCompetitorName.String(cn); ok {
		c.Name = value
	}
	if value, ok := xpCompetitorClub.String(cn); ok {
		c.Club = value
	}
	if value, ok := xpCompetitorAgeClass.String(cn); ok {
		c.AgeClass = value
	}
	if value, ok := xpCompetitorTime.String(cn); ok {
		compTime, err := decodeTime(value)
		if err != nil {
			c.Valid = false
		}
		c.Time = compTime
	}

	/*
		<tr>
			<td class="wrap">1st</td>
			<td>Mike Edwards</td>
			<td>RUNANDFALLOVER</td>
			<td class="centre">N</td>
			<td class="right">15:50</td>
		</tr>
	*/

	return &c, nil
}

var reHourMinuteSecond *regexp.Regexp
var reMinuteSecond *regexp.Regexp
var reSecond *regexp.Regexp

func decodeTime(t string) (time.Duration, error) {
	if matches := reHourMinuteSecond.FindStringSubmatch(t); matches != nil {
		hours := asciiToDuration(matches[1])
		minutes := asciiToDuration(matches[2])
		seconds := asciiToDuration(matches[3])
		return seconds*time.Second + minutes*time.Minute + hours*time.Hour, nil
	}
	if matches := reMinuteSecond.FindStringSubmatch(t); matches != nil {
		minutes := asciiToDuration(matches[1])
		seconds := asciiToDuration(matches[2])
		return seconds*time.Second + minutes*time.Minute, nil
	}
	if matches := reSecond.FindStringSubmatch(t); matches != nil {
		seconds := asciiToDuration(matches[1])
		return seconds * time.Second, nil
	}
	return 0, errors.New("NO MATCH")
}

func asciiToDuration(s string) time.Duration {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return time.Duration(val)
}

func initDecode() {
	xpTitle = xmlpath.MustCompile("/html/body/div[@id='container']/div[@id='container_b']/div[@id='container_bp']/h2")
	xpCourse = xmlpath.MustCompile("/html/body/div[@id='container']/div[@id='container_b']/div[@id='container_bp']/fieldset[1]/div[@class='resultsblock']")

	xpCourseTitle = xmlpath.MustCompile("div[@class='resultsblock-title']/h2")
	xpCourseInfo = xmlpath.MustCompile("p[@class='resultsblock-info']")
	xpCourseCompetitors = xmlpath.MustCompile("table[@class='data_wide']/tbody/tr")

	xpCompetitorPosition = xmlpath.MustCompile("td[1]")
	xpCompetitorName = xmlpath.MustCompile("td[2]")
	xpCompetitorClub = xmlpath.MustCompile("td[3]")
	xpCompetitorAgeClass = xmlpath.MustCompile("td[4]")
	xpCompetitorTime = xmlpath.MustCompile("td[5]")

	reValidPosition = regexp.MustCompile(`^\W*[0-9]+(st|nd|rd|th)\W*$`)

	reHourMinuteSecond = regexp.MustCompile(`^\W*([0-9]+):([0-9]+):([0-9]+)\W*$`)
	reMinuteSecond = regexp.MustCompile(`^\W*([0-9]+):([0-9]+)\W*$`)
	reSecond = regexp.MustCompile(`^\W*([0-9]+)\W*$`)

}
