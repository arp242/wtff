package wtff

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"zgo.at/zstd/zbyte"
	"zgo.at/zstd/zfilepath"
)

// Print all ffmpeg commands to stderr
var ShowFFCmd = false

// Cut a part and write to output.
func Cut(ctx context.Context, input, output string, start, stop Time) error {
	out, err := ffmpeg(ctx,
		// "-stats",
		"-ss", start.String(), // Stream before opening
		"-i", input, // Input
		"-to", stop.String(), // Duration
		"-avoid_negative_ts", "make_zero",
		"-map_metadata", "0",
		"-movflags", "+faststart",
		"-default_mode", "infer_no_subs",
		"-c", "copy",
		output).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.Probe: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	return nil
}

// Cat all files to the output, without re-encoding.
func Cat(ctx context.Context, output string, input ...string) error {
	tmp, err := os.CreateTemp("", "wtff.*")
	if err != nil {
		return err
	}
	metaTmp, err := os.CreateTemp("", "wtff.*")
	if err != nil {
		return err
	}

	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
		os.Remove(metaTmp.Name())
	}()

	var (
		m Meta
		l time.Duration
	)
	if len(input) == 1 {
		m, err = ReadMeta(ctx, input[0])
		if err != nil {
			return err
		}
		abs, err := filepath.Abs(input[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(tmp, "file '%s'\n", strings.ReplaceAll(abs, `'`, `'\''`))
		if err != nil {
			return err
		}
	} else {
		for _, i := range input {
			p, err := Probe(ctx, i)
			if err != nil {
				return err
			}

			i, err := filepath.Abs(i)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(tmp, "file '%s'\n", strings.ReplaceAll(i, `'`, `'\''`))
			if err != nil {
				return err
			}
			t, _ := zfilepath.SplitExt(filepath.Base(i))
			m.Chapters = append(m.Chapters, MetaChapter{
				Timebase: [2]int64{1, 1000},
				Start:    l.Milliseconds(),
				End:      (l + p.Format.Duration.Duration).Milliseconds(),
				Title:    t,
			})
			l += p.Format.Duration.Duration
		}
	}
	err = tmp.Close()
	if err != nil {
		return err
	}

	_, err = metaTmp.WriteString(m.String())
	if err != nil {
		return err
	}
	err = metaTmp.Close()
	if err != nil {
		return err
	}

	// TODO: H.264 is buggy: https://trac.ffmpeg.org/ticket/9893
	out, err := ffmpeg(ctx,
		"-f", "concat",
		"-safe", "0", // Trust filenames
		"-i", tmp.Name(),
		"-i", metaTmp.Name(),
		"-map_chapters", "1",
		"-map_metadata", "1",
		"-c", "copy",
		output).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.Probe: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	return nil
}

func SubAdd(ctx context.Context, input, subFile, lang string) error {
	tmp, err := tmpFile(input)
	if err != nil {
		return fmt.Errorf("wtff.SubAdd: %w", err)
	}
	defer os.Remove(tmp.Name())

	info, err := Probe(ctx, input)
	if err != nil {
		return fmt.Errorf("wtff.SubAdd: %w", err)
	}
	codec := "srt"
	if info.Format.FormatName == "mov,mp4,m4a,3gp,3g2,mj2" {
		codec = "mov_text"
	}
	var n int
	for _, s := range info.Streams {
		if s.Subtitle() {
			n++
		}
	}

	out, err := ffmpeg(ctx,
		"-y",
		"-i", input,
		"-i", subFile,
		"-map", "0",
		"-map", "1",
		"-c", "copy",
		"-c:s", codec,
		"-metadata:s:s:"+strconv.Itoa(n), "language="+lang,
		tmp.Name()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.SubAdd: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	err = os.Rename(tmp.Name(), input)
	if err != nil {
		return fmt.Errorf("wtff.SubAdd: %w", err)
	}
	return nil
}

func SubRm(ctx context.Context, input, stream string) error {
	tmp, err := tmpFile(input)
	if err != nil {
		return fmt.Errorf("wtff.SubRm: %w", err)
	}
	defer os.Remove(tmp.Name())

	info, err := Probe(ctx, input)
	if err != nil {
		return fmt.Errorf("wtff.SubRm: %w", err)
	}

	args := []string{
		"-y",
		"-i", input,
		"-map", "0:v",
		"-map", "0:a",
		//"-map_metadata",
	}
	if stream != "ALL" {
		rm := info.Streams.Find("subtitle", stream)
		if rm == -1 {
			return fmt.Errorf("stream %q not found or not a subtitle", stream)
		}
		for _, s := range info.Streams {
			if s.Subtitle() && s.Index != rm {
				args = append(args, "-map", "0:"+strconv.Itoa(s.Index))
			}
		}
	}
	args = append(args, "-c", "copy", tmp.Name())

	out, err := ffmpeg(ctx, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.SubRm: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	err = os.Rename(tmp.Name(), input)
	if err != nil {
		return fmt.Errorf("wtff.SubRm: %w", err)
	}
	return nil
}

func SubSave(ctx context.Context, input, stream, output string, overwrite bool) error {
	info, err := Probe(ctx, input)
	if err != nil {
		return fmt.Errorf("wtff.SubSave: %w", err)
	}

	n := info.Streams.Find("subtitle", stream)
	if n == -1 {
		return fmt.Errorf("stream %q not found or not a subtitle", stream)
	}

	args := []string{"-i", input, "-map", "0:" + strconv.Itoa(n), output}
	if overwrite {
		args = append(args, "-y")
	}

	out, err := ffmpeg(ctx, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.SubSave: %w: %s", err, zbyte.ElideLeft(out, 500))
	}

	return nil
}

func AudioAdd(ctx context.Context, input, audioFile, lang, title string) error {
	tmp, err := tmpFile(input)
	if err != nil {
		return fmt.Errorf("wtff.AudioAdd: %w", err)
	}
	defer os.Remove(tmp.Name())

	info, err := Probe(ctx, input)
	if err != nil {
		return fmt.Errorf("wtff.AudioAdd: %w", err)
	}
	var n int
	for _, s := range info.Streams {
		if s.Audio() {
			n++
		}
	}

	// TODO: look into -shortest and -apad
	out, err := ffmpeg(ctx,
		"-y",
		"-i", input,
		"-i", audioFile,
		"-map", "0",
		"-map", "1",
		"-c", "copy",
		"-metadata:s:a:"+strconv.Itoa(n), "language="+lang,
		"-metadata:s:a:"+strconv.Itoa(n), "title="+title,
		tmp.Name()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.AudioAdd: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	err = os.Rename(tmp.Name(), input)
	if err != nil {
		return fmt.Errorf("wtff.AudioAdd: %w", err)
	}
	return nil
}

func AudioRm(ctx context.Context, input, stream string) error {
	tmp, err := tmpFile(input)
	if err != nil {
		return fmt.Errorf("wtff.AudioRm: %w", err)
	}
	defer os.Remove(tmp.Name())

	info, err := Probe(ctx, input)
	if err != nil {
		return fmt.Errorf("wtff.AudioRm: %w", err)
	}

	args := []string{
		"-y",
		"-i", input,
		"-map", "0:v",
		"-map", "0:s",
	}
	if stream != "ALL" {
		rm := info.Streams.Find("audio", stream)
		if rm == -1 {
			return fmt.Errorf("stream %q not found or not a audio track", stream)
		}
		for _, s := range info.Streams {
			if s.Audio() && s.Index != rm {
				args = append(args, "-map", "0:"+strconv.Itoa(s.Index))
			}
		}
	}
	args = append(args, "-c", "copy", tmp.Name())

	out, err := ffmpeg(ctx, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.AudioRm: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	err = os.Rename(tmp.Name(), input)
	if err != nil {
		return fmt.Errorf("wtff.AudioRm: %w", err)
	}
	return nil
}

func AudioSave(ctx context.Context, input, stream, output string) error {
	info, err := Probe(ctx, input)
	if err != nil {
		return fmt.Errorf("wtff.AudioSave: %w", err)
	}

	n := info.Streams.Find("audio", stream)
	if n == -1 {
		return fmt.Errorf("stream %q not found or not a audio track", stream)
	}

	out, err := ffmpeg(ctx,
		"-i", input,
		"-c", "copy",
		"-map", "0:"+strconv.Itoa(n),
		output).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.AudioSave: %w: %s", err, zbyte.ElideLeft(out, 500))
	}

	return nil
}

func tmpFile(path string) (*os.File, error) {
	base, ext := zfilepath.SplitExt(path)
	return os.CreateTemp(filepath.Dir(path), filepath.Base(base)+"-wtff-meta-*."+ext)
}

func ffmpeg(ctx context.Context, args ...string) *exec.Cmd {
	args = append([]string{"-hide_banner", "-v", "warning"}, args...)
	if ShowFFCmd {
		qa := make([]string, 0, len(args))
		for _, a := range args {
			qa = append(qa, shellQuote(a))
		}
		fmt.Fprintf(os.Stderr, "ffmpeg "+strings.Join(args, " ")+"\n")
	}
	return exec.CommandContext(ctx, "ffmpeg", args...)
}

func shellQuote(s string) string {
	if len(s) == 0 {
		return "''"
	}
	if !strings.ContainsAny(s, "\\'\"`${[|&;<>()*?! \t\n") && s[0] != '~' {
		return s
	}
	if strings.Contains(s, "'") && !strings.ContainsAny("\\\"$`", s) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
