package query

import (
	"fmt"
	"testing"
)

func TestListeningClockDates(t *testing.T) {

	tables := []struct {
		InMonth  int
		InYear   int
		Start    string
		End      string
		StartAvg string
		// EndAvg is always equal to Start
	}{
		{4, 2019, "2019-04-01", "2019-05-01", "2018-10-01"},
		{7, 2019, "2019-07-01", "2019-08-01", "2019-01-01"},
		{1, 2019, "2019-01-01", "2019-02-01", "2018-07-01"},
	}

	for _, test := range tables {
		fmt.Println(test)

		start1, end1, start2 := listeningClockDates(test.InMonth, test.InYear)
		if start1 != test.Start {
			t.Errorf("wrong start date - got:%s expected:%s", start1, test.Start)
		}
		if end1 != test.End {
			t.Errorf("wrong end date - got:%s expected:%s", end1, test.End)
		}
		if start2 != test.StartAvg {
			t.Errorf("wrong start avg date - got:%s expected:%s", start2, test.StartAvg)
		}
	}

}
