package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"zgo.at/wtff"
)

func cmdCut(input, output, verb, startStr, endStr string) error {
	start, err := parseTime(startStr)
	if err != nil {
		return fmt.Errorf("invalid: %q: %s", startStr, err)
	}
	stop, err := parseTime(endStr)
	if err != nil {
		return fmt.Errorf("invalid: %q: %s", endStr, err)
	}
	switch strings.ToLower(verb) {
	default:
		return fmt.Errorf("invalid: %q", verb)
	case "to":
		stop = wtff.Time{stop.Duration - start.Duration}
	case "for":
		// Do nothing.
	}
	return wtff.Cut(context.Background(), input, output, start, stop)
}

func parseTime(s string) (wtff.Time, error) {
	sp := strings.Split(s, ":")
	if len(sp) < 2 {
		return wtff.Time{}, errors.New("not enough :")
	} else if len(sp) > 3 {
		return wtff.Time{}, errors.New("too many :")
	}
	var sub time.Duration
	if i := strings.IndexByte(sp[len(sp)-1], '.'); i > -1 {
		var x string
		sp[len(sp)-1], x, _ = strings.Cut(sp[len(sp)-1], ".")
		var err error
		sub, err = time.ParseDuration("0." + x + "s")
		if err != nil {
			return wtff.Time{}, err
		}
	}

	a, err := strconv.Atoi(sp[0])
	if err != nil {
		return wtff.Time{}, err
	}
	b, err := strconv.Atoi(sp[1])
	if err != nil {
		return wtff.Time{}, err
	}
	if len(sp) == 2 {
		return wtff.Time{sub + time.Duration(a)*time.Minute + time.Duration(b)*time.Second}, nil
	}

	c, err := strconv.Atoi(sp[2])
	if err != nil {
		return wtff.Time{}, err
	}
	return wtff.Time{sub + time.Duration(a)*time.Hour + time.Duration(b)*time.Minute + time.Duration(c)*time.Second}, nil
}
