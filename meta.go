package wtff

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"zgo.at/zstd/zbyte"
	"zgo.at/zstd/zmap"
)

// TODO: much of this can/should probably be merged in to probe.go. This
// originally came from two different scripts and got merged here, but it's kind
// of redundant.

type (
	Meta struct {
		Comment          string            `toml:"-"`
		MajorBrand       string            `toml:"-"`
		MinorVersion     string            `toml:"-"`
		CompatibleBrands string            `toml:"-"`
		Encoder          string            `toml:"-"`
		Title            string            `toml:"title"`
		Artist           string            `toml:"artist"`
		Date             string            `toml:"date"`
		Chapters         []MetaChapter     `toml:"chapters"`
		Other            map[string]string `toml:"other"`
	}
	MetaChapter struct {
		Timebase  [2]int64 `toml:"-"`
		Start     int64    `toml:"-"`
		End       int64    `toml:"-"`
		Title     string   `toml:"title"`
		TOMLStart string   `toml:"start"`
	}
)

func (m MetaChapter) StartSecs() int {
	return int(time.Duration(m.Start * (1_000_000_000 / m.Timebase[1])).Seconds())
}

func ReadMeta(ctx context.Context, input string) (Meta, error) {
	out, err := ffmpeg(ctx,
		"-v", "error",
		"-i", input,
		"-f", "ffmetadata", "-").CombinedOutput()
	if err != nil {
		return Meta{}, fmt.Errorf("wtff.ReadMeta: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	m, err := ParseMeta(string(out))
	m.Comment = input
	return m, err
}

func ParseMetaFromTOML(input string) (Meta, error) {
	var m Meta
	_, err := toml.Decode(input, &m)
	if err != nil {
		return m, err
	}

	for i, c := range m.Chapters {
		m.Chapters[i].Timebase = [2]int64{1, 1000}
		sp := strings.Split(c.TOMLStart, ":")
		switch len(sp) {
		case 2:
			a, err := strconv.Atoi(sp[0])
			if err != nil {
				return m, err
			}
			b, err := strconv.Atoi(sp[1])
			if err != nil {
				return m, err
			}
			m.Chapters[i].Start = int64(a*60+b) * 1000
		case 3:
			a, err := strconv.Atoi(sp[0])
			if err != nil {
				return m, err
			}
			b, err := strconv.Atoi(sp[1])
			if err != nil {
				return m, err
			}
			c, err := strconv.Atoi(sp[2])
			if err != nil {
				return m, err
			}
			m.Chapters[i].Start = int64(a*3600+b*60+c) * 1000
		default:
			return m, fmt.Errorf("invalid start: %q", c)
		}
		if i > 0 && m.Chapters[i-1].Start > m.Chapters[i].Start {
			return m, fmt.Errorf("chapter %d (%q) starts after the preceding", i+1, c.Title)
		}
	}

	// End *needs* to be set, even if it's just the next chapter. Weird Shit™
	// happens if you don't.
	for i := range m.Chapters {
		if i == len(m.Chapters)-1 {
			// Should be to end of file, but we don't have that here. This seems
			// to work well.
			//
			// TODO: this fails on mkv files:
			//   [matroska @ 0x56456d3ac380] Invalid chapter start (6228000000000) or end (-9223372036854775808)
			// Should pass input file and get length.
			m.Chapters[i].End = math.MaxInt64
		} else {
			m.Chapters[i].End = m.Chapters[i+1].Start
		}
	}

	return m, err
}

func ParseMeta(input string) (Meta, error) {
	m := Meta{Other: make(map[string]string)}

	lines := strings.Split(input, "\n")
	if lines[0] != ";FFMETADATA1" {
		return m, fmt.Errorf("wtff.ParseMeta: unexpected start: %q", lines[0])
	}

	var (
		chapters bool
		chapter  MetaChapter
		skip     int
	)
	for i, line := range lines[1:] {
		if skip > 0 {
			skip--
			continue
		}
		if line == "Output file is empty, nothing was encoded " || line == "" {
			continue
		}
		if strings.ToLower(line) == "[chapter]" {
			if chapter.Title != "" {
				m.Chapters = append(m.Chapters, chapter)
				chapter = MetaChapter{}
			}
			chapters = true
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return m, fmt.Errorf("wtff.ParseMeta: line %d: %q", i+1, line)
		}

		var err error
		if chapters {
			switch strings.ToLower(k) {
			case "timebase":
				a, b, ok := strings.Cut(v, "/")
				if !ok {
					return m, fmt.Errorf("wtff.ParseMeta: line %d: invalid timebase: %q", i+1, v)
				}
				aa, err := strconv.ParseInt(a, 10, 64)
				if err != nil {
					return m, fmt.Errorf("wtff.ParseMeta: line %d: %v", i+1, err)
				}
				bb, err := strconv.ParseInt(b, 10, 64)
				if err != nil {
					return m, fmt.Errorf("wtff.ParseMeta: line %d: %v", i+1, err)
				}

				chapter.Timebase = [2]int64{aa, bb}
			case "start":
				chapter.Start, err = strconv.ParseInt(v, 10, 64)
				if err != nil {
					return m, fmt.Errorf("wtff.ParseMeta: line %d: %v", i+1, err)
				}
			case "end":
				chapter.End, err = strconv.ParseInt(v, 10, 64)
				if err != nil {
					return m, fmt.Errorf("wtff.ParseMeta: line %d: %v", i+1, err)
				}
			case "title":
				chapter.Title = v
			}
			continue
		}

		if strings.HasSuffix(v, `\`) {
			for {
				line = lines[i+skip+1]
				if skip > 0 {
					v += "\n" + line
				}
				if !strings.HasSuffix(line, `\`) {
					break
				}
				v = v[:len(v)-1]
				skip++
			}
		}

		switch strings.ToLower(k) {
		case "major_brand":
			m.MajorBrand = v
		case "minor_version":
			m.MinorVersion = v
		case "compatible_brands":
			m.CompatibleBrands = v
		case "title":
			m.Title = v
		case "artist":
			m.Artist = v
		case "date":
			m.Date = v
		case "encoder":
			m.Encoder = v
		default:
			m.Other[k] = v
		}
	}
	if chapter.Title != "" {
		m.Chapters = append(m.Chapters, chapter)
	}
	return m, nil
}

func WriteMeta(ctx context.Context, m Meta, input, output string) error {
	tmp, err := os.CreateTemp("", "ffmeta-*.txt")
	if err != nil {
		return fmt.Errorf("wtff.WriteMeta: %w", err)
	}
	defer os.Remove(tmp.Name())

	_, err = tmp.WriteString(m.String())
	if err != nil {
		return fmt.Errorf("wtff.WriteMeta: %w", err)
	}
	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("wtff.WriteMeta: %w", err)
	}
	out, err := ffmpeg(ctx,
		"-y",
		"-i", input,
		"-i", tmp.Name(),
		"-map_chapters", "1",
		"-map_metadata", "1",
		"-codec", "copy",
		"-movflags", "+use_metadata_tags", // Magic flag to make freeform MP4 tags work
		output).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wtff.WriteMeta: %w: %s", err, zbyte.ElideLeft(out, 500))
	}
	return nil
}

func (m Meta) String() string {
	b := new(strings.Builder)
	b.WriteString(";FFMETADATA1\n")

	if m.MajorBrand != "" {
		fmt.Fprintf(b, "major_brand=%s\n", m.MajorBrand)
	}
	if m.MinorVersion != "" {
		fmt.Fprintf(b, "minor_version=%s\n", m.MinorVersion)
	}
	if m.CompatibleBrands != "" {
		fmt.Fprintf(b, "compatible_brands=%s\n", m.CompatibleBrands)
	}
	if m.Encoder != "" {
		fmt.Fprintf(b, "encoder=%s\n", m.Encoder)
	}
	if m.Title != "" {
		fmt.Fprintf(b, "title=%s\n", strings.ReplaceAll(m.Title, "\n", "\\\n"))
	}
	if m.Artist != "" {
		fmt.Fprintf(b, "artist=%s\n", strings.ReplaceAll(m.Artist, "\n", "\\\n"))
	}
	if m.Date != "" {
		fmt.Fprintf(b, "date=%s\n", m.Date)
	}
	for _, k := range zmap.KeysOrdered(m.Other) {
		fmt.Fprintf(b, "%s=%s\n", k, strings.ReplaceAll(m.Other[k], "\n", "\\\n"))
	}

	for _, c := range m.Chapters {
		b.WriteString("[CHAPTER]\n")
		fmt.Fprintf(b, "TIMEBASE=%d/%d\n", c.Timebase[0], c.Timebase[1])
		fmt.Fprintf(b, "START=%d\n", c.Start)
		fmt.Fprintf(b, "END=%d\n", c.End)
		fmt.Fprintf(b, "title=%s\n", c.Title)
	}
	return b.String()
}

func tomlStr(w io.Writer, width int, key string, val string) {
	enc := toml.NewEncoder(w)
	if strings.Contains(key, " ") {
		key = "'" + key + "'"
	}
	fmt.Fprintf(w, "%-"+strconv.Itoa(width)+"s = ", key)
	if strings.Contains(val, "\n") {
		fmt.Fprintf(w, `""" %s """`, val)
	} else {
		enc.Encode(val)
	}
	w.Write([]byte{'\n'})
}

func (m Meta) TOML() string {
	b := new(strings.Builder)

	if m.Comment != "" {
		fmt.Fprintf(b, "# %s\n\n", m.Comment)
	}
	// fmt.Fprintf(b, "# major_brand       = %s\n", m.MajorBrand)
	// fmt.Fprintf(b, "# minor_version     = %s\n", m.MinorVersion)
	// fmt.Fprintf(b, "# compatible_brands = %s\n", m.CompatibleBrands)
	// fmt.Fprintf(b, "# encoder           = %s\n\n", m.Encoder)

	tomlStr(b, 12, "artist", m.Artist)
	tomlStr(b, 12, "title", m.Title)
	tomlStr(b, 12, "date", m.Date)
	b.WriteString("chapters     = [\n")
	enc := toml.NewEncoder(b)
	pad := ""
	for _, c := range m.Chapters {
		if c.StartSecs() > 3600 {
			pad = "   "
			break
		}
	}

	for _, c := range m.Chapters {
		s := c.StartSecs()
		t := fmt.Sprintf(`%s"%02d:%02d"`, pad, s/60, s%60)
		if s > 3600 {
			t = fmt.Sprintf(`"%02d:%02d:%02d"`, s/3600, s/60%60, s%60)
		}
		fmt.Fprintf(b, `    {start = %s, title = `, t)
		enc.Encode(c.Title)
		b.WriteString("},\n")
	}
	if len(m.Chapters) == 0 {
		b.WriteString(`    # {start = "00:00", title = ""},` + "\n")
	}
	b.WriteString("]\n")

	if len(m.Other) > 0 {
		b.WriteString("[other]\n")
		l := zmap.LongestKey(m.Other) + 2
		for _, k := range zmap.KeysOrdered(m.Other) {
			b.WriteString("    ")
			tomlStr(b, l, k, m.Other[k])
		}
		b.WriteString("\n")
	}

	return b.String()
}
