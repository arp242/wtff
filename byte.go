package wtff

import (
	"fmt"
	"strconv"
)

type Byte float64

var units = []string{"B", "K", "M", "G", "T", "P"}

func (b Byte) String() string {
	i := 0
	for ; i < len(units); i++ {
		if b < 1024 {
			return fmt.Sprintf("%.1f%s", b, units[i])
		}
		b /= 1024
	}
	return fmt.Sprintf("%.1f%s", b*1024, units[i-1])
}

// TODO: finish and move to zstd (it's enough for our purpose here).
func (b *Byte) UnmarshalText(in []byte) error {
	s := string(in)
	switch {
	// case strings.HasSuffix(s, "B"):
	// case strings.HasSuffix(s, "K"):
	// case strings.HasSuffix(s, "M"):
	// case strings.HasSuffix(s, "G"):
	// case strings.HasSuffix(s, "T"):
	// case strings.HasSuffix(s, "P"):
	default:
		sz, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		*b = Byte(sz)
	}
	return nil
}
