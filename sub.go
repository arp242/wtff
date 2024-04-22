package wtff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Adopted from https://github.com/chiflix/subtitles/blob/master/srt.go

type (
	SubLine struct {
		Seq   int
		Start time.Time
		End   time.Time
		Text  []string
	}
	Subs []SubLine
)

func (s Subs) String() string {
	var b strings.Builder
	b.Grow(1024)
	for _, ss := range s {
		b.WriteString(ss.String())
	}
	return b.String()
}

func (s SubLine) String() string {
	b := new(strings.Builder)
	b.Grow(128)
	fmt.Fprintf(b, "%d\n%s --> %s\n", s.Seq, fmtTime(s.Start), fmtTime(s.End))
	for _, line := range s.Text {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

var reSRT = regexp.MustCompile(`([0-9]+:*[0-9]+:[0-9]+[\.,]+[0-9]+)\s+-->\s+([0-9]+:*[0-9]+:[0-9]+[\.,]+[0-9]+)`)

func ParseSRT(s string) (Subs, error) {
	var (
		res    = make(Subs, 0, 512)
		lines  = strings.Split(s, "\n")
		outSeq = 1
	)
	for i, line := range lines {
		matches := reSRT.FindStringSubmatch(stripSpaces(line))
		if len(matches) < 3 {
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			_, err := strconv.Atoi(line)
			if err == nil {
				/// Skip this seq number if the next line is timecode
				if i+1 < len(lines) && len(reSRT.FindStringSubmatch(stripSpaces(lines[i+1]))) >= 3 {
					continue
				}
			}
			/// Not time codes, so it may be text
			if l := len(res) - 1; l >= 0 {
				res[l].Text = append(res[l].Text, line)
			}
			continue
		}

		o := SubLine{Seq: outSeq}
		var err error
		o.Start, err = parseTime(matches[1])
		if err != nil {
			return Subs{}, fmt.Errorf("srt: start error at line %d: %w", i, err)
		}

		o.End, err = parseTime(matches[2])
		if err != nil {
			return Subs{}, fmt.Errorf("srt: end error at line %d: %w", i, err)
		}

		if removeLastEmptyCaption(&res, &o) {
			outSeq--
		} else {
			res = append(res, o)
			outSeq++
		}
	}

	removeLastEmptyCaption(&res, nil)
	return res, nil
}

func stripSpaces(line string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Number, unicode.Symbol, unicode.Punct, unicode.Dash, unicode.White_Space) {
			return r
		}
		return -1
	}, line)
}

func removeLastEmptyCaption(res *Subs, o *SubLine) bool {
	ll := len(*res)
	if ll > 0 && len((*res)[ll-1].Text) == 0 {
		if o != nil {
			o.Seq--
			(*res)[ll-1] = *o
		} else {
			*res = (*res)[:ll-1]
		}
		return true
	}
	return false
}

func fmtTime(t time.Time) string {
	return strings.Replace(t.Format("15:04:05.000"), ".", ",", 1)
}

var reTime = regexp.MustCompile("([0-9]+):([0-9]+):([0-9]+):([0-9]+)")

// parseTime parses a subtitle time (duration since start of film)
func parseTime(in string) (time.Time, error) {
	// . and , to :
	in = strings.Replace(in, ",", ":", -1)
	in = strings.Replace(in, ".", ":", -1)
	if strings.Count(in, ":") == 1 {
		in = "00:" + in
	}
	if strings.Count(in, ":") == 2 {
		in += ":000"
	}

	matches := reTime.FindStringSubmatch(in)
	if len(matches) < 5 {
		return time.Time{}, fmt.Errorf("[srt] Regexp didnt match: %s", in)
	}
	h, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, err
	}
	m, err := strconv.Atoi(matches[2])
	if err != nil {
		return time.Time{}, err
	}
	s, err := strconv.Atoi(matches[3])
	if err != nil {
		return time.Time{}, err
	}
	ms, err := strconv.Atoi(matches[4])
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(0, 1, 1, h, m, s, ms*1000*1000, time.UTC), nil
}
