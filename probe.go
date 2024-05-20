package wtff

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"zgo.at/zstd/zbyte"
)

func (t Time) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *Time) UnmarshalText(b []byte) error {
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	t.Duration = time.Duration(f) * time.Second
	return nil
}

func (t Time) String() string {
	h, m, s := t.Truncate(time.Hour).Hours(), t.Truncate(time.Minute).Minutes(), t.Truncate(time.Second).Seconds()
	f := fmt.Sprintf("%02d:%02d", int(m)%60, int(s)%60)
	if h > 0 {
		f = fmt.Sprintf("%02d:%s", int(h), f)
	}
	r := t.Duration % time.Second
	if r > 0 {
		f = strings.TrimRight(fmt.Sprintf("%s.%d", f, r), "0")
	}
	return f
}

type (
	Time      struct{ time.Duration }
	ProbeFile struct {
		Format   Format   `json:"format"`
		Streams  Streams  `json:"streams"`
		Chapters Chapters `json:"chapters"`
	}
	Streams  []Stream
	Chapters []Chapter
	Format   struct {
		Filename       string         `json:"filename,omitempty"`         // "01.01 Snooper Force.mkv"
		NbStreams      uint           `json:"nb_streams,omitempty"`       // 3
		NbPrograms     uint           `json:"nb_programs,omitempty"`      // 0
		FormatName     string         `json:"format_name,omitempty"`      // "matroska,webm"
		FormatLongName string         `json:"format_long_name,omitempty"` // "Matroska / WebM"
		StartTime      Time           `json:"start_time,omitempty"`       // "0.000000"
		Duration       Time           `json:"duration,omitempty"`         // "1746.601000"
		Size           Byte           `json:"size,omitempty"`             // "324196219"
		BitRate        string         `json:"bit_rate,omitempty"`         // "1484924"
		ProbeScore     uint           `json:"probe_score,omitempty"`      // 100
		Tags           map[string]any `json:"tags,omitempty"`
	}
	Stream struct {
		Index              int    `json:"index,omitempty"`                // 0
		CodecName          string `json:"codec_name,omitempty"`           // "hevc"
		CodecLongName      string `json:"codec_long_name,omitempty"`      // "H.265 / HEVC (High Efficiency Video Coding)"
		Profile            string `json:"profile,omitempty"`              // "Main 10"
		CodecType          string `json:"codec_type,omitempty"`           // "video"
		CodecTagString     string `json:"codec_tag_string,omitempty"`     // "[0][0][0][0]"
		CodecTag           string `json:"codec_tag,omitempty"`            // "0x0000"
		Width              uint   `json:"width,omitempty"`                // 720
		Height             uint   `json:"height,omitempty"`               // 568
		CodedWidth         uint   `json:"coded_width,omitempty"`          // 720
		CodedHeight        uint   `json:"coded_height,omitempty"`         // 568
		ClosedCaptions     uint   `json:"closed_captions,omitempty"`      // 0
		HasBFrames         uint   `json:"has_b_frames,omitempty"`         // 2
		SampleAspectRatio  string `json:"sample_aspect_ratio,omitempty"`  // "349:240"
		DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"` // "1047:568"
		PixFmt             string `json:"pix_fmt,omitempty"`              // "yuv420p10le"
		Level              int    `json:"level,omitempty"`                // 90, -99?
		ColorRange         string `json:"color_range,omitempty"`          // "tv"
		ChromaLocation     string `json:"chroma_location,omitempty"`      // "left"
		Refs               uint   `json:"refs,omitempty"`                 // 1
		RFrameRate         string `json:"r_frame_rate,omitempty"`         // "25/1"
		AvgFrameRate       string `json:"avg_frame_rate,omitempty"`       // "25/1"
		TimeBase           string `json:"time_base,omitempty"`            // "1/1000"
		StartPts           uint   `json:"start_pts,omitempty"`            // 80
		StartTime          string `json:"start_time,omitempty"`           // "0.080000"

		SampleFmt     string `json:"sample_fmt,omitempty"`      // "fltp"
		SampleRate    string `json:"sample_rate,omitempty"`     // "48000"
		Channels      uint   `json:"channels,omitempty"`        // 2
		ChannelLayout string `json:"channel_layout,omitempty"`  // "stereo"
		BitsPerSample uint   `json:"bits_per_sample,omitempty"` // 0

		BitRate    string `json:"bit_rate,omitempty"`    // "2498735"
		DurationTs uint   `json:"duration_ts,omitempty"` // 1746601
		Duration   Time   `json:"duration,omitempty"`    // "1746.601000"

		Disposition struct {
			Default         uint `json:"default,omitempty"`
			Dub             uint `json:"dub,omitempty"`
			Original        uint `json:"original,omitempty"`
			Comment         uint `json:"comment,omitempty"`
			Lyrics          uint `json:"lyrics,omitempty"`
			Karaoke         uint `json:"karaoke,omitempty"`
			Forced          uint `json:"forced,omitempty"`
			HearingImpaired uint `json:"hearing_impaired,omitempty"`
			VisualImpaired  uint `json:"visual_impaired,omitempty"`
			CleanEffects    uint `json:"clean_effects,omitempty"`
			AttachedPic     uint `json:"attached_pic,omitempty"`
			TimedThumbnails uint `json:"timed_thumbnails,omitempty"`
		} `json:"disposition,omitempty"`
		Tags map[string]any `json:"tags,omitempty"`
	}
	Chapter struct {
		Id        int            `json:"id,omitempty"`         // -1349333221,
		TimeBase  string         `json:"time_base,omitempty"`  // "1/1000000000",
		Start     int            `json:"start,omitempty"`      // 0,
		StartTime Time           `json:"start_time,omitempty"` // "0.000000",
		End       uint           `json:"end,omitempty"`        // 335400000000,
		EndTime   Time           `json:"end_time,omitempty"`   // "335.400000",
		Tags      map[string]any `json:"tags,omitempty"`       // {"title": "Chapter 01"}
	}
)

func (p ProbeFile) String() string {
	b := new(strings.Builder)
	for _, s := range p.Streams {
		var nfo []string
		if s.Width > 0 {
			nfo = append(nfo, fmt.Sprintf("%d×%d", s.Width, s.Height))
		}
		if l, ok := s.Tags["language"].(string); ok && l != "" && l != "und" {
			nfo = append(nfo, l)
		}
		sz := Byte(0)
		if t, ok := s.Tags["NUMBER_OF_BYTES"].(string); ok && t != "" {
			sz.UnmarshalText([]byte(t))
		} else if t, ok := s.Tags["NUMBER_OF_BYTES-eng"].(string); ok && t != "" {
			sz.UnmarshalText([]byte(t))
		} else if s.BitRate != "" && s.Duration.Duration > 0 {
			b, _ := strconv.ParseUint(s.BitRate, 10, 64)
			sz = Byte(float64(b) / 8 * s.Duration.Seconds())
		}
		psz := ""
		if sz > 0 {
			psz = sz.String()
		}
		var t string
		if tt, ok := s.Tags["title"].(string); ok && tt != "" {
			//t = ", " + tt
			t = "\n" + strings.Repeat(" ", 52) + "title=" + tt
		}
		fmt.Fprintf(b, "%-3v %-9s %-13s %-16s %6s %s%s\n", s.Index, s.CodecType,
			s.CodecName, strings.Join(nfo, ", "), psz, s.CodecLongName, t)
	}
	if len(p.Chapters) > 0 {
		b.WriteString("Chapters:\n")
	}
	for _, c := range p.Chapters {
		t := "[untitled]"
		if tt, ok := c.Tags["title"].(string); ok && tt != "" {
			t = tt
		}
		fmt.Fprintf(b, "    %8s  %8s    %s\n", c.StartTime, c.EndTime, t)
	}
	if b.Len() > 0 {
		return b.String()[:b.Len()-1]
	}
	return ""
}

// Probe gets an overview of streams for this file.
func Probe(ctx context.Context, file string) (ProbeFile, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-hide_banner", "-v", "quiet",
		"-of", "json=compact=1",
		"-show_error", "-show_format", "-show_streams", "-show_chapters",
		file).CombinedOutput()
	if err != nil {
		var outj struct {
			Error struct {
				Code   int    `json:"code"`
				String string `json:"string"`
			} `json:"error"`
		}
		errj := json.Unmarshal(out, &outj)
		if errj == nil {
			return ProbeFile{}, fmt.Errorf("wtff.Probe: %w: %s (code %d)", err, outj.Error.String, outj.Error.Code)
		}
		return ProbeFile{}, fmt.Errorf("wtff.Probe: %w: %s", err, zbyte.ElideLeft(out, 500))
	}

	var p ProbeFile
	err = json.Unmarshal(out, &p)
	if err != nil {
		return ProbeFile{}, fmt.Errorf("wtff.Probe: %w", err)
	}
	return p, nil
}

func (s Stream) Subtitle() bool { return s.CodecType == "subtitle" }
func (s Stream) Video() bool    { return s.CodecType == "video" }
func (s Stream) Audio() bool    { return s.CodecType == "audio" }

func (s Streams) Find(kind, langOrNum string) int {
	streamN, err := strconv.Atoi(langOrNum)
	if err != nil {
		streamN = -1
	}
	for _, ss := range s {
		if ss.CodecType != kind {
			continue
		}
		if streamN >= -1 && ss.Index == streamN {
			return ss.Index
		}
		if streamN == -1 {
			lang, ok := ss.Tags["language"].(string)
			if ok && lang == langOrNum {
				return ss.Index
			}
		}
	}
	return -1
}
