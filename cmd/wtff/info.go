package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"zgo.at/wtff"
	"zgo.at/zli"
	"zgo.at/zstd/zmap"
)

func cmdInfo(meta bool, files ...string) error {
	multi := len(files) > 1

	for i, file := range files {
		info, err := wtff.Probe(context.Background(), file)
		if !multi && err != nil { /// For multi print the errors below path and continue
			return err
		}
		if multi && i > 0 {
			fmt.Println()
		}
		zli.Colorf(file, zli.Bold)
		fmt.Printf(" (%s, %s)\n", info.Format.Duration, info.Format.Size)

		if err != nil {
			zli.Errorf(err)
		}
		if multi {
			fmt.Println("\t" + strings.ReplaceAll(info.String(), "\n", "\n\t"))
		} else {
			fmt.Println(info)
		}
		if meta {
			m, err := wtff.ReadMeta(context.Background(), file)
			if !multi && err != nil {
				return err
			}
			if err != nil {
				zli.Errorf(err)
			}
			b := new(strings.Builder)
			prMeta(b, 12, "title", m.Title)
			prMeta(b, 12, "artist", m.Artist)
			prMeta(b, 12, "date", m.Date)
			if len(m.Other) > 0 {
				b.WriteByte('\n')
			}
			l := zmap.LongestKey(m.Other)
			for _, k := range zmap.KeysOrdered(m.Other) {
				prMeta(b, l, k, m.Other[k])
			}
			if b.Len() > 0 {
				fmt.Println("Meta:")
				fmt.Print(b.String())
			}
		}
	}
	return nil
}

func prMeta(b *strings.Builder, w int, k, v string) {
	if v == "" {
		return
	}
	fmt.Fprintf(b, "    %-"+strconv.Itoa(w)+"s = ", k)
	if strings.Contains(v, "\n") {
		fmt.Fprintln(b, "\n"+"        "+strings.ReplaceAll(v, "\n", "\n        "))
	} else {
		fmt.Fprintln(b, v)
	}
}
