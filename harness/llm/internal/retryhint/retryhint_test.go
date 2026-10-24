package retryhint

import (
	"net/http"
	"testing"
	"time"
)

func TestNumber(t *testing.T) {
	for _, unit := range []time.Duration{time.Second, time.Millisecond} {
		for _, test := range []struct {
			value string
			want  time.Duration
		}{
			{" 2.5 ", unit * 5 / 2}, {"1e2", 100 * unit}, {"0", 0}, {"-1", 0},
			{"NaN", 0}, {"+Inf", 0}, {"Inf", 0}, {"-Inf", 0}, {"-1e999", 0}, {"1e-999", 0}, {"bad", 0},
			{"1e300", maxDelay}, {"1e999", maxDelay}, {"999999999999999999999999", maxDelay},
		} {
			t.Run(test.value+unit.String(), func(t *testing.T) {
				if got := Number(test.value, unit); got != test.want {
					t.Fatalf("got %s want %s", got, test.want)
				}
			})
		}
	}
	for _, test := range []struct {
		value string
		want  time.Duration
	}{
		{"9223372036", 9223372036 * time.Second}, {"9223372037", maxDelay},
		{"9223372036.854776", maxDelay}, {"9223372036.854775808", maxDelay},
		{"11.054", 11054 * time.Millisecond},
	} {
		if got := Number(test.value, time.Second); got != test.want {
			t.Errorf("%s: got %s want %s", test.value, got, test.want)
		}
	}
}

func TestHeaderDates(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		value string
		want  time.Duration
	}{
		{now.Add(time.Hour).Format(http.TimeFormat), time.Hour},
		{now.Format(http.TimeFormat), 0}, {now.Add(-time.Second).Format(http.TimeFormat), 0},
		{"Fri, 31 Dec 9999 23:59:59 GMT", maxDelay},
		{"not a date", 0}, {"2.5", 2500 * time.Millisecond},
	} {
		if got := Header(test.value, now); got != test.want {
			t.Errorf("%s: got %s want %s", test.value, got, test.want)
		}
	}
}
