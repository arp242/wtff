package main

import (
	"errors"
	"fmt"
	"os"

	"zgo.at/wtff"
	"zgo.at/zli"
)

var usageBrief = `
wtff is a frontend for ffmpeg. https://github.com/arp242/wtff

Commands:
    info         [-m] [file] [file...]
    meta         [-w] [-s] [-t toml-file] [file]
    mb           [-artist artist] [-album album] [-r release-id] [file]
    cat          [-f] [-o output] [input...]
    cut          [-o output] [input] [start] [verb] [stop]
    sub add      [-l lang] [input] [sub-file]
    sub rm       [input] [stream]
    sub save     [input] [stream] [output]
    sub print    [input] [stream]
    audio add    [-l lang] [-t title] [input] [audio-file]
    audio rm     [input] [stream]
    audio save   [-o output] [input] [stream]

Use the -v flag with any command to print the ffmpeg invocations to stderr.

Use "help" or "-h" for full help.
`[1:]

var usage = `
wtff is a frontend for ffmpeg. https://github.com/arp242/wtff

Use the -v flag with any command to print the ffmpeg invocations to stderr.

Commands:
    info [-m] [file] [file...]
            Show list of streams and chapters for all given files. This is
            similar to ffprobe, but more compat and excludes most metadata.

            Flags:
                -m, -meta      Display more metadata.

    meta [-w] [-s] [-t toml-file] [file]
            Edit metadata as a TOML file with $EDITOR. Keys can be deleted to
            remove that data.
            Writes to temp file in the file's directory, and overwrites the
            original on success.

            Flags:
                -s, -strip     Strip all metadata except chapters; does not open
                               $EDITOR and immediately writes. If given twice
                               also remove all chapters.
                -t, -toml      Start $EDITOR with the given TOML file, instead
                               of populating from metadata from the input file.
                -w, -write     Write without starting $EDITOR; only makes sense
                               if -t is set.

    mb [-artist artist] [-album album] [-r release-id] [file]
             Load metadata from MusicBrainz and open $EDITOR (as with the meta
             command). This assumes that every album is exactly one file, rather
             than one file per track. This is how I store my music, which is a
             bit non-standard but works quite well. For more standard usage
             you're probably better off with Picard or something.

             Assumes the filename is in the format "artist - album.ext", but you
             can override these with the -artist and -album flags.

             Or alternatively, use -r release-id to directly load a release
             instead of searching. That's the ID in:
             https://musicbrainz.org/release/68395b54-0890-3d70-b031-8103824b073a

    cat [-f] [-o output] [input...]
            Cat all the input files to the output without recoding data.
            Filenames are added as chapters (if the format supports it).

            The files must be compatible: same streams, with same encoding, with
            same parameters (e.g. resolution). There are some basic checks for
            this, but it's not guaranteed to be comprehensive. May give wonky
            results if they're not identical

            This can also be used to change the container format when used with
            just one input file; for example to change a AVI to MP4:
                % wtff file.avi file.mp4

            Flags:
                -f, -force     Force operation, even if files look incompatible.

    cut [-o output] [input] [start] [verb] [stop]
           Cut a pieces from a file:

                00:01:33  to  00:01:40   Explicit start/stop times.
                01:33.123 to  01:40.123  Sub-second, omitting hour
                01:33.123 for 00:01:00   for 1 minute

    sub add [-l lang] [input] [sub-file]
           Add a new subtitle from file; [lang] is optional and should be the
           3-letter language code (e.g. eng).

    sub rm [input] [stream]
           Remove subtitle from a file; the stream can either be a stream number
           (as reported in 'wtff info') or language. Will remove all matching
           subtitles when using a language name. Use the stream "ALL" to remove
           all subtitles.

    sub save [-o output] [input] [stream]
           Save subtitle to file.

    sub print [input] [stream]
           Print subtitle to stdout.

    audio add [-l lang] [-t title] [input] [audio-file]
           Add a new audio track from audio-file.

    audio rm [input] [stream]
           Remove audio track from a file; the stream can either be a stream
           number (as reported in 'wtff info') or language. Will remove all
           matching tracks when using a language name. Use the stream "ALL" to
           remove all audio tracks.

    audio save [-o output] [input] [stream]
           Save audio to file.
`[1:]

func main() {
	f := zli.NewFlags(os.Args)
	var (
		helpFlag    = f.Bool(false, "h", "help")
		verboseFlag = f.Bool(false, "v", "verbose")
	)
	zli.F(f.Parse(zli.AllowUnknown()))
	if helpFlag.Bool() {
		fmt.Print(usage)
		return
	}
	if verboseFlag.Bool() {
		wtff.ShowFFCmd = true
	}
	cmd, err := f.ShiftCommand("help", "info", "meta", "mb", "cut", "cat", "subs", "audio")
	if errors.Is(err, zli.ErrCommandNoneGiven{}) {
		fmt.Print(usageBrief)
		return
	}
	zli.F(err)

	var cmdErr error
	switch cmd {
	case "help":
		fmt.Print(usage)
	case "info":
		var (
			meta = f.Bool(false, "m", "meta")
		)
		zli.F(f.Parse())
		if len(f.Args) == 0 {
			zli.Fatalf(`"info" command needs at least one file`)
		}
		cmdErr = cmdInfo(meta.Bool(), f.Args...)
	case "meta":
		var (
			tomlFile = f.String("", "t", "toml-file")
			write    = f.Bool(false, "w", "write")
			strip    = f.IntCounter(0, "s", "strip")
		)
		zli.F(f.Parse())
		if write.Set() && !tomlFile.Set() {
			zli.Fatalf("-w needs -t")
		}
		if strip.Int() > 0 && (write.Set() || tomlFile.Set()) {
			zli.Fatalf("-s can't be combined with -w or -t")
		}
		if len(f.Args) != 1 {
			zli.Fatalf(`"meta" command needs exactly one input file`)
		}
		cmdErr = cmdMeta(f.Args[0], tomlFile.String(), !write.Bool(), strip.Int())
	case "mb":
		var (
			artist  = f.String("", "artist")
			album   = f.String("", "album")
			release = f.String("", "r", "release")
		)
		zli.F(f.Parse())
		if len(f.Args) != 1 {
			zli.Fatalf(`"mb" command needs exactly one input file`)
		}
		cmdErr = cmdMb(f.Args[0], artist.String(), album.String(), release.String())
	case "cat":
		var (
			force  = f.Bool(false, "f", "force")
			output = f.String("", "-o", "output")
		)
		zli.F(f.Parse())
		if output.String() == "" {
			zli.Fatalf("need to set output file with -o")
		}
		if len(f.Args) < 1 {
			zli.Fatalf("need at least one input file")
		}
		cmdErr = cmdCat(force.Bool(), output.String(), f.Args...)
	case "cut":
		var (
			output = f.String("", "-o", "output")
		)
		zli.F(f.Parse())
		if output.String() == "" {
			zli.Fatalf("need to set output file with -o")
		}
		if len(f.Args) != 4 {
			zli.Fatalf("usage: wtff cut [-o output] [input] [start] [verb] [stop]")
		}
		cmdErr = cmdCut(f.Args[0], output.String(), f.Args[2], f.Args[2], f.Args[4])
	case "subs":
		subCmd, err := f.ShiftCommand("add", "rm", "save", "replace", "print", "sync", "burn")
		zli.F(err)
		cmdErr = cmdSub(f, subCmd)
	case "audio":
		subCmd, err := f.ShiftCommand("add", "rm", "save", "replace")
		zli.F(err)
		cmdErr = cmdAudio(f, subCmd)
	}
	zli.F(cmdErr)
}
