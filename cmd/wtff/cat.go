package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"zgo.at/wtff"
)

func cmdCat(force bool, output string, input ...string) error {
	if !force {
		formats := make([]string, 0, len(input))
		for _, f := range input {
			info, err := wtff.Probe(context.Background(), f)
			if err != nil {
				return err
			}

			f := make([]string, 0, 4)
			for _, s := range info.Streams {
				if s.CodecType != "subtitle" && s.CodecType != "data" {
					f = append(f, fmt.Sprintf("%s %s %dx%d", s.CodecType, s.CodecName, s.Width, s.Height))
				}
			}
			formats = append(formats, strings.Join(f, "\n"))
		}
		same := true
		for i := range formats {
			if i > 0 && formats[i] != formats[i-1] {
				same = false
				break
			}
		}
		if !same {
			fmt.Println("Formats not identical:")
			for i := range formats {
				fmt.Println("  " + filepath.Base(input[i]) + "\n" + "    " + strings.ReplaceAll(formats[i], "\n", "\n    "))
			}
			fmt.Println("\nEdit files to make the streams compatible (if possible), or use -f to force.")
			return nil
		}
	}

	return wtff.Cat(context.Background(), output, input...)
}
