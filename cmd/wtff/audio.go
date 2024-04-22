package main

import (
	"context"

	"zgo.at/wtff"
	"zgo.at/zli"
)

func cmdAudio(f zli.Flags, cmd string) error {
	switch cmd {
	case "add":
		var (
			lang  = f.String("", "l", "lang")
			title = f.String("", "t", "title")
		)
		zli.F(f.Parse())
		if len(f.Args) != 2 && len(f.Args) != 3 && len(f.Args) != 4 {
			zli.Fatalf("usage: wtff audio add [-l lang] [-t title] [media] [audio-file]")
		}
		return cmdAudioAdd(f.Args[0], f.Args[1], lang.String(), title.String())
	case "rm":
		zli.F(f.Parse())
		if len(f.Args) != 2 {
			zli.Fatalf("usage: wtff audio rm [media] [stream]")
		}
		return cmdAudioRm(f.Args[0], f.Args[1])
	case "save":
		var (
			output = f.String("", "o", "output")
		)
		zli.F(f.Parse())
		if len(f.Args) != 3 {
			zli.Fatalf("usage: wtff audio save [-o output] [media] [stream]")
		}
		return cmdAudioSave(f.Args[0], f.Args[1], output.String())
	case "replace":
		zli.F(f.Parse())
		return nil // TODO
	}
	panic("unreachable")
}

func cmdAudioAdd(input, audioFile, lang, title string) error {
	return wtff.AudioAdd(context.Background(), input, audioFile, lang, title)
}

func cmdAudioRm(input, stream string) error {
	return wtff.AudioRm(context.Background(), input, stream)
}

func cmdAudioSave(input, stream, output string) error {
	return wtff.AudioSave(context.Background(), input, stream, output)
}
