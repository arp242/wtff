package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"zgo.at/wtff"
	"zgo.at/zstd/zfilepath"
)

// TODO: allow editing per-stream meta
func cmdMeta(input, tomlFile string, editFile bool) error {
	var (
		oktoRm    = true
		tomlMeta  string
		doErr     = func(err error) error { return err }
		runEditor = func(p string) error {
			editor := "vi"
			if e, ok := os.LookupEnv("EDITOR"); ok {
				editor = e
			}

			cmd := exec.CommandContext(context.Background(), editor, p)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		}
	)
	if tomlFile == "" {
		m, err := wtff.ReadMeta(context.Background(), input)
		if err != nil {
			return err
		}

		tmp, err := os.CreateTemp("", "wtff-meta-*.toml")
		if err != nil {
			return err
		}
		defer func() {
			if oktoRm {
				os.Remove(tmp.Name())
			}
		}()

		_, err = tmp.WriteString(m.TOML())
		if err != nil {
			return err
		}
		err = tmp.Close()
		if err != nil {
			return err
		}

		err = runEditor(tmp.Name())
		if err != nil {
			return err
		}

		doErr = func(err error) error { return fmt.Errorf("%w\nTOML file saved in %q", err, tmp.Name()) }
		oktoRm = false
		n, err := os.ReadFile(tmp.Name())
		if err != nil {
			return doErr(err)
		}
		tomlMeta = string(n)
	} else {
		if editFile {
			err := runEditor(tomlFile)
			if err != nil {
				return err
			}
		}
		n, err := os.ReadFile(tomlFile)
		if err != nil {
			return err
		}
		tomlMeta = string(n)
	}

	m, err := wtff.ParseMetaFromTOML(tomlMeta)
	if err != nil {
		return doErr(err)
	}

	base, ext := zfilepath.SplitExt(input)
	outTmp, err := os.CreateTemp(filepath.Dir(input), filepath.Base(base)+"-wtff-meta-*."+ext)
	if err != nil {
		return err
	}
	defer os.Remove(outTmp.Name())

	err = wtff.WriteMeta(context.Background(), m, input, outTmp.Name())
	if err != nil {
		return doErr(err)
	}

	err = os.Rename(outTmp.Name(), input)
	if err != nil {
		return doErr(err)
	}

	oktoRm = true
	return nil
}

func cmdMb(input, artist, album, release string) error {
	inp, _ := zfilepath.SplitExt(filepath.Base(input))
	aa, al, ok := strings.Cut(inp, " - ")
	if !ok {
		album = artist
	}
	if artist == "" {
		artist = aa
	}
	if album == "" {
		album = al
	}

	m, err := wtff.ReadMeta(context.Background(), input)
	if err != nil {
		return err
	}
	info, err := wtff.Probe(context.Background(), input)
	if err != nil {
		return err
	}
	s := int(info.Format.Duration.Seconds())
	m.Comment += fmt.Sprintf("  length=%02d:%02d", s/60, s%60)

	var out string
	if release == "" {
		out, err = mbSearch(artist, album)
		if err != nil {
			return err
		}
	} else {
		r, err := mbRelease(release)
		if err != nil {
			return err
		}
		out = r.Meta().TOML()
	}

	fp, err := os.CreateTemp("", "wtff-mb-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(fp.Name())

	var skip bool
	for _, l := range strings.Split(m.TOML()+"\n"+out, "\n") {
		if skip {
			skip = false
			continue
		}
		if len(l) > 0 && l[0] == '#' {
			skip = true
		} else if len(l) > 0 {
			l = "    " + l
		}
		_, err = fp.WriteString(l + "\n")
		if err != nil {
			return err
		}
	}

	err = fp.Close()
	if err != nil {
		return err
	}

	return cmdMeta(input, fp.Name(), true)
}

type (
	MBSearch struct {
		Releases []struct {
			ID    string `json:"id"`    // "7d4cc770-4790-4814-a578-82890dadfa57"
			Score uint8  `json:"score"` // 100
		} `json:"releases"`
	}
	MBRelease struct {
		ID           string `json:"id"`    // "7d4cc770-4790-4814-a578-82890dadfa57"
		Title        string `json:"title"` // "When the Kite String Pops"
		Date         string `json:"date"`  // "1994-08-08"
		ArtistCredit []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"artist-credit"`
		Media []struct {
			Tracks []struct {
				ID       string `json:"id"`       // "dcb4c94f-ff93-3af6-9aa1-bc92406efadd"
				Position uint16 `json:"position"` // 1
				Number   string `json:"number"`   // "1"
				Length   uint   `json:"length"`   // 373066
				Title    string `json:"title"`    // "The Blue"
			} `json:"tracks"`
		} `json:"media"`
	}
)

func mbReq(u string, args ...any) ([]byte, error) {
	u = fmt.Sprintf(u, args...)
	if wtff.ShowFFCmd {
		fmt.Println(u)
	}
	r, _ := http.NewRequest("GET", u, nil)
	r.Header.Set("Accept", "application/json; charset=utf-8")
	r.Header.Set("User-Agent", "wtff/1.0.0 ( https://github.com/arp242/wtff )")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return out, err
}

var escQuery = newEscaper(`\`, strings.Split(`+ - && || ! ( ) { } [ ] ^ " ~ * ? : \ /`, " ")...)

func tomlStr(s string) string {
	b := new(strings.Builder)
	b.Grow(len(s) + 4)
	err := toml.NewEncoder(b).Encode(s)
	if err != nil {
		panic(err)
	}
	return b.String()
}

func mbSearch(artist, album string) (string, error) {
	out, err := mbReq("https://musicbrainz.org/ws/2/release?query=%s&limit=10&offset=0", url.QueryEscape(
		fmt.Sprintf(`artist:"%s" AND release:"%s"`, escQuery.Replace(artist), escQuery.Replace(album))))
	if err != nil {
		return "", err
	}
	var s MBSearch
	err = json.Unmarshal(out, &s)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, ss := range s.Releases {
		r, err := mbRelease(ss.ID)
		if err != nil {
			return "", err
		}
		m := r.Meta()
		m.Comment += fmt.Sprintf("  score=%d", ss.Score)
		b.WriteString(m.TOML())
	}

	return strings.TrimSpace(b.String()), nil
}

func mbRelease(relID string) (MBRelease, error) {
	out, err := mbReq("https://musicbrainz.org/ws/2/release/%s?inc=recordings+artist-credits", relID)
	if err != nil {
		return MBRelease{}, err
	}
	var r MBRelease
	err = json.Unmarshal(out, &r)
	if err != nil {
		return MBRelease{}, err
	}
	return r, nil
}

func (r MBRelease) Meta() wtff.Meta {
	artist := make([]string, 0, 2)
	for _, a := range r.ArtistCredit {
		artist = append(artist, a.Name)
	}
	m := wtff.Meta{
		Artist: strings.Join(artist, ", "),
		Title:  r.Title,
		Date:   r.Date,
	}
	var i, start int64
	for _, md := range r.Media {
		for _, t := range md.Tracks {
			i++
			m.Chapters = append(m.Chapters, wtff.MetaChapter{
				Timebase: [2]int64{1, 1000},
				Start:    start,
				Title:    fmt.Sprintf("%02d %s", i, t.Title),
			})
			start += int64(t.Length)
		}
	}

	m.Comment = fmt.Sprintf("https://musicbrainz.org/release/%s  length=%s",
		r.ID, fmt.Sprintf("%02d:%02d", start/1000/60, start/1000%60))
	m.Other = map[string]string{"MusicBrainz Album Id": r.ID}
	return m
}

// TODO: move to zstring

type escaper struct{ repl *strings.Replacer }

func (e *escaper) Replace(s string) string { return e.repl.Replace(s) }

func newEscaper(escString string, escSet ...string) *escaper {
	args := make([]string, 0, len(escSet)*2)
	for _, e := range escSet {
		args = append(args, e, escString+e)
	}
	r := strings.NewReplacer(args...)
	return &escaper{repl: r}
}
