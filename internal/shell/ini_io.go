package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

type EncodingKind string

const (
	EncodingUTF8BOM   EncodingKind = "utf8-bom"
	EncodingUTF8NoBOM EncodingKind = "utf8"
	EncodingUTF16LE   EncodingKind = "utf16le"
	EncodingUTF16BE   EncodingKind = "utf16be"
	EncodingCP932     EncodingKind = "cp932"
)

type TextFile struct {
	Text     string
	Encoding EncodingKind
}

func ReadTextFile(path string) (TextFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TextFile{}, err
	}
	return decodeText(data)
}

func WriteTextFile(path string, src TextFile) error {
	encoded, err := encodeText(src)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}

func EnsureBackup(path string, backupDir string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	base := filepath.Base(path)
	stamp := time.Now().Format("20060102_150405")
	dst := filepath.Join(backupDir, fmt.Sprintf("%s.%s.bak", base, stamp))
	return dst, copyFile(path, dst)
}

func RestoreBackup(src string, dst string) error {
	return copyFile(src, dst)
}

func decodeText(data []byte) (TextFile, error) {
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xEF, 0xBB, 0xBF}) {
		return TextFile{
			Text:     string(data[3:]),
			Encoding: EncodingUTF8BOM,
		}, nil
	}
	if len(data) >= 2 && bytes.Equal(data[:2], []byte{0xFF, 0xFE}) {
		txt, err := decodeUTF16(data[2:], true)
		return TextFile{Text: txt, Encoding: EncodingUTF16LE}, err
	}
	if len(data) >= 2 && bytes.Equal(data[:2], []byte{0xFE, 0xFF}) {
		txt, err := decodeUTF16(data[2:], false)
		return TextFile{Text: txt, Encoding: EncodingUTF16BE}, err
	}
	if utf8.Valid(data) {
		return TextFile{
			Text:     string(data),
			Encoding: EncodingUTF8NoBOM,
		}, nil
	}
	decoder := japanese.ShiftJIS.NewDecoder()
	decoded, err := decoder.Bytes(data)
	if err != nil {
		return TextFile{}, err
	}
	return TextFile{
		Text:     string(decoded),
		Encoding: EncodingCP932,
	}, nil
}

func encodeText(src TextFile) ([]byte, error) {
	if src.Encoding == EncodingUTF8BOM {
		return append([]byte{0xEF, 0xBB, 0xBF}, []byte(src.Text)...), nil
	}
	if src.Encoding == EncodingUTF8NoBOM || src.Encoding == "" {
		return []byte(src.Text), nil
	}
	if src.Encoding == EncodingUTF16LE {
		return encodeUTF16(src.Text, true), nil
	}
	if src.Encoding == EncodingUTF16BE {
		return encodeUTF16(src.Text, false), nil
	}
	if src.Encoding == EncodingCP932 {
		encoder := japanese.ShiftJIS.NewEncoder()
		return encoder.Bytes([]byte(src.Text))
	}
	return nil, fmt.Errorf("未対応エンコード: %s", src.Encoding)
}

func decodeUTF16(data []byte, littleEndian bool) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("UTF-16 byte length is invalid")
	}
	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		a := uint16(data[i])
		b := uint16(data[i+1])
		value := (a << 8) | b
		if littleEndian {
			value = (b << 8) | a
		}
		u16 = append(u16, value)
	}
	runes := utf16.Decode(u16)
	return string(runes), nil
}

func encodeUTF16(text string, littleEndian bool) []byte {
	runes := []rune(text)
	u16 := utf16.Encode(runes)
	out := make([]byte, 0, 2+len(u16)*2)
	if littleEndian {
		out = append(out, 0xFF, 0xFE)
	}
	if !littleEndian {
		out = append(out, 0xFE, 0xFF)
	}
	for _, value := range u16 {
		hi := byte(value >> 8)
		lo := byte(value)
		if littleEndian {
			out = append(out, lo, hi)
		}
		if !littleEndian {
			out = append(out, hi, lo)
		}
	}
	return out
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if strings.TrimSpace(dst) == "" {
		return fmt.Errorf("backup path is empty")
	}
	return out.Sync()
}
