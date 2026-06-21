package utils

import (
	"encoding/csv"
	"io"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CleanExportText repairs common UTF-8-as-Windows-1252 mojibake, normalizes
// typography to export-safe ASCII punctuation, and removes pictographic
// symbols that are not reliably supported by PDF/CSV consumers.
func CleanExportText(value string) string {
	cleaned := value
	for i := 0; i < 4 && containsMojibakeMarker(cleaned); i++ {
		decoded, ok := decodeWindows1252AsUTF8(cleaned)
		if !ok || decoded == cleaned {
			break
		}
		cleaned = decoded
	}

	cleaned = strings.NewReplacer(
		"\u2010", "-",
		"\u2011", "-",
		"\u2012", "-",
		"\u2013", "-",
		"\u2014", "-",
		"\u2212", "-",
		"\u2018", "'",
		"\u2019", "'",
		"\u201c", "\"",
		"\u201d", "\"",
		"\u2026", "...",
		"\u00a0", " ",
	).Replace(cleaned)

	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		switch {
		case r == utf8.RuneError || r == '\u00c2':
			continue
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(r)
		case unicode.IsControl(r) || unicode.In(r, unicode.Cf):
			continue
		case isPictographicSymbol(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func CleanCSVRecord(record []string) []string {
	cleaned := make([]string, len(record))
	for i, value := range record {
		cleaned[i] = strings.TrimSpace(CleanExportText(value))
	}
	return cleaned
}

type SanitizedCSVWriter struct {
	*csv.Writer
}

func NewSanitizedCSVWriter(writer io.Writer) *SanitizedCSVWriter {
	return &SanitizedCSVWriter{Writer: csv.NewWriter(writer)}
}

func (w *SanitizedCSVWriter) Write(record []string) error {
	return w.Writer.Write(CleanCSVRecord(record))
}

type SanitizingWriter struct {
	Writer io.Writer
}

func NewSanitizingWriter(writer io.Writer) *SanitizingWriter {
	return &SanitizingWriter{Writer: writer}
}

func (w *SanitizingWriter) Write(p []byte) (int, error) {
	cleaned := []byte(CleanExportText(string(p)))
	_, err := w.Writer.Write(cleaned)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// SanitizeExportData mutates string fields inside export-only DTOs. It is
// intentionally called at export boundaries so stored accounting data remains
// untouched while generated PDF/Excel content stays clean.
func SanitizeExportData(data interface{}) {
	sanitizeReflectValue(reflect.ValueOf(data), make(map[uintptr]bool), 0)
}

func sanitizeReflectValue(value reflect.Value, visited map[uintptr]bool, depth int) {
	if !value.IsValid() || depth > 24 {
		return
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return
		}
		inner := value.Elem()
		if inner.Kind() == reflect.String && value.CanSet() {
			value.Set(reflect.ValueOf(CleanExportText(inner.String())))
			return
		}
		sanitizeReflectValue(inner, visited, depth+1)
		return
	}
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return
		}
		pointer := value.Pointer()
		if pointer != 0 && visited[pointer] {
			return
		}
		if pointer != 0 {
			visited[pointer] = true
		}
		sanitizeReflectValue(value.Elem(), visited, depth+1)
		return
	}

	switch value.Kind() {
	case reflect.String:
		if value.CanSet() {
			value.SetString(strings.TrimSpace(CleanExportText(value.String())))
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.CanInterface() {
				sanitizeReflectValue(field, visited, depth+1)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			sanitizeReflectValue(value.Index(i), visited, depth+1)
		}
	case reflect.Map:
		if value.IsNil() {
			return
		}
		iterator := value.MapRange()
		for iterator.Next() {
			key := iterator.Key()
			item := iterator.Value()
			if item.Kind() == reflect.Interface && !item.IsNil() && item.Elem().Kind() == reflect.String {
				value.SetMapIndex(key, reflect.ValueOf(strings.TrimSpace(CleanExportText(item.Elem().String()))))
			} else if item.Kind() == reflect.String {
				value.SetMapIndex(key, reflect.ValueOf(strings.TrimSpace(CleanExportText(item.String()))).Convert(item.Type()))
			} else {
				sanitizeReflectValue(item, visited, depth+1)
			}
		}
	}
}

func containsMojibakeMarker(value string) bool {
	return strings.ContainsAny(value, "ÃÂâðï")
}

func decodeWindows1252AsUTF8(value string) (string, bool) {
	bytes := make([]byte, 0, len(value))
	for _, r := range value {
		if r <= 0xff {
			bytes = append(bytes, byte(r))
			continue
		}
		b, ok := windows1252Byte(r)
		if !ok {
			return value, false
		}
		bytes = append(bytes, b)
	}
	if !utf8.Valid(bytes) {
		return value, false
	}
	return string(bytes), true
}

func windows1252Byte(r rune) (byte, bool) {
	lookup := map[rune]byte{
		'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84, '…': 0x85,
		'†': 0x86, '‡': 0x87, 'ˆ': 0x88, '‰': 0x89, 'Š': 0x8a,
		'‹': 0x8b, 'Œ': 0x8c, 'Ž': 0x8e, '‘': 0x91, '’': 0x92,
		'“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97,
		'˜': 0x98, '™': 0x99, 'š': 0x9a, '›': 0x9b, 'œ': 0x9c,
		'ž': 0x9e, 'Ÿ': 0x9f,
	}
	b, ok := lookup[r]
	return b, ok
}

func isPictographicSymbol(r rune) bool {
	if r == '\ufe0e' || r == '\ufe0f' || r == '\u200d' || r == '\u20e3' {
		return true
	}
	if unicode.In(r, unicode.So) {
		return true
	}
	return r >= 0x1f000 && r <= 0x1faff
}
