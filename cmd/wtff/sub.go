package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"zgo.at/wtff"
	"zgo.at/zli"
)

func cmdSub(f zli.Flags, cmd string) error {
	switch cmd {
	case "add":
		var (
			lang = f.String("", "l", "lang")
		)
		zli.F(f.Parse())
		if len(f.Args) != 2 {
			zli.Fatalf("usage: wtff sub add [-l lang] [media] [sub-file]")
		}
		return cmdSubAdd(f.Args[0], f.Args[1], lang.String())
	case "rm":
		zli.F(f.Parse())
		if len(f.Args) != 2 {
			zli.Fatalf("usage: wtff sub rm [media] [stream]")
		}
		return cmdSubRm(f.Args[0], f.Args[1])
	case "save":
		var (
			output = f.String("", "o", "output")
		)
		zli.F(f.Parse())
		if len(f.Args) != 2 {
			zli.Fatalf("usage: wtff sub save [-o output] [media] [stream]")
		}
		return cmdSubSave(f.Args[0], f.Args[1], output.String())
	case "print":
		zli.F(f.Parse())
		if len(f.Args) != 2 {
			zli.Fatalf("usage: wtff sub print [media] [stream]")
		}
		return cmdSubPrint(f.Args[0], f.Args[1])
	case "replace":
		zli.F(f.Parse())
		return nil // TODO
	case "sync":
		zli.F(f.Parse())
		data, err := os.ReadFile(f.Args[0])
		zli.F(err)
		sub, err := wtff.ParseSRT(string(data))
		zli.F(err)

		d := 150 * time.Millisecond
		for i := range sub {
			sub[i].Start = sub[i].Start.Add(d)
			sub[i].End = sub[i].End.Add(d)
		}
		// TODO: write it. Ideally, "sub add" should read from stdin:
		//
		// sub sync file.srt +200ms | sub add file.mp4 -
		// sub sync file.srt +200ms | sub replace file.mp4 -
		// sub print file.mp4 | sub sync - +200ms | sub replace file.mp4
		fmt.Println(sub)
		return nil
	case "burn":
		// TODO
		return nil
	}
	panic("unreachable")
}

func cmdSubAdd(input, subFile, lang string) error {
	nosub := []string{".avi"}
	if i := slices.Index(nosub, filepath.Ext(input)); i > -1 {
		return fmt.Errorf("%q format does not support subtitles", nosub[i])
	}
	return wtff.SubAdd(context.Background(), input, subFile, lang)
}

func cmdSubRm(input, stream string) error {
	return wtff.SubRm(context.Background(), input, stream)
}

func cmdSubSave(input, stream, output string) error {
	return wtff.SubSave(context.Background(), input, stream, output, false)
}

func cmdSubPrint(input, stream string) error {
	tmp, err := os.CreateTemp("", "wtff.*.srt")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	err = wtff.SubSave(context.Background(), input, stream, tmp.Name(), true)
	if err != nil {
		return err
	}

	sub, err := os.ReadFile(tmp.Name())
	if err != nil {
		return err
	}
	fmt.Println(string(bytes.TrimRight(sub, "\n")))

	return nil
}
