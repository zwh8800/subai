package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/asticode/go-astisub"
)

type SubtitleItem struct {
	Index   int
	StartAt time.Duration
	EndAt   time.Duration
	Text    string
	Chinese string
}

type Subtitle struct {
	Items []*SubtitleItem
}

func ParseSubtitle(filePath string) (*Subtitle, error) {
	s, err := astisub.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subtitle file: %w", err)
	}

	sub := &Subtitle{
		Items: make([]*SubtitleItem, 0, len(s.Items)),
	}

	for _, item := range s.Items {
		lines := make([]string, len(item.Lines))
		for i, line := range item.Lines {
			lines[i] = line.String()
		}
		subItem := &SubtitleItem{
			Index:   item.Index,
			StartAt: item.StartAt,
			EndAt:   item.EndAt,
			Text:    strings.Join(lines, "\n"),
		}
		sub.Items = append(sub.Items, subItem)
	}

	return sub, nil
}

func (s *Subtitle) GenerateSRT() string {
	var builder strings.Builder
	for _, item := range s.Items {
		builder.WriteString(fmt.Sprintf("%d\n", item.Index))
		builder.WriteString(formatTime(item.StartAt))
		builder.WriteString(" --> ")
		builder.WriteString(formatTime(item.EndAt))
		builder.WriteString("\n")

		if item.Chinese != "" {
			builder.WriteString(item.Chinese)
			builder.WriteString("\n")
		}
		builder.WriteString(item.Text)
		builder.WriteString("\n\n")
	}
	return builder.String()
}

func (s *Subtitle) GenerateASS() string {
	var builder strings.Builder
	builder.WriteString("[Script Info]\n")
	builder.WriteString("ScriptType: v4.00+\n")
	builder.WriteString("Collisions: Normal\n")
	builder.WriteString("PlayDepth: 0\n\n")

	builder.WriteString("[V4+ Styles]\n")
	builder.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	builder.WriteString("Style: Chinese,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,10,10,10,1\n")
	builder.WriteString("Style: English,Arial,16,&H0080B2C2,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,10,10,10,1\n\n")

	builder.WriteString("[Events]\n")
	builder.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	for _, item := range s.Items {
		if item.Chinese != "" {
			chinese := fmt.Sprintf("{\\rChinese}%s", escapeASSText(item.Chinese))
			english := fmt.Sprintf("{\\rEnglish}%s", escapeASSText(item.Text))
			text := chinese + "\\N" + english
			builder.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Chinese,,0,0,0,,%s\n",
				formatASSTime(item.StartAt),
				formatASSTime(item.EndAt),
				text))
		} else {
			text := fmt.Sprintf("{\\rEnglish}%s", escapeASSText(item.Text))
			builder.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,English,,0,0,0,,%s\n",
				formatASSTime(item.StartAt),
				formatASSTime(item.EndAt),
				text))
		}
	}

	return builder.String()
}

func formatTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func formatASSTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	centis := int(d.Milliseconds()/10) % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centis)
}

func escapeASSText(text string) string {
	replacer := strings.NewReplacer(
		"\\\\", "\\\\\\\\",
		"{", "\\{",
		"}", "\\}",
		"\n", "\\N",
	)
	return replacer.Replace(text)
}

func saveToFile(filePath, content string) error {
	return os.WriteFile(filePath, []byte(content), 0644)
}
