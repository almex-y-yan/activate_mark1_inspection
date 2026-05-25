package core

import (
	"fmt"
	"regexp"
	"strings"
)

const AutoCommentPrefix = ";AUTO_OFF "

var (
	sectionHeaderPattern = regexp.MustCompile(`^\[[^\]]+\]$`)
	comPattern           = regexp.MustCompile(`^\s*Com\s*=\s*([0-9]+)`)
	comPrefixPattern     = regexp.MustCompile(`^\s*Com\s*=\s*`)
)

type PatchResult struct {
	Changed bool
	Text    string
}

type sectionRange struct {
	Start int
	End   int
}

func GetSectionComValue(text string, sectionName string) (*int, error) {
	lines, _ := splitLines(text)
	target, found := findSectionRange(lines, sectionName)
	if !found {
		return nil, fmt.Errorf("[%s]なし", sectionName)
	}

	matches := make([]int, 0, 2)
	for idx := target.Start; idx <= target.End; idx++ {
		line := removeManagedCommentPrefix(lines[idx])
		trim := strings.TrimSpace(line)
		if isCommentLine(trim) {
			continue
		}
		captured := comPattern.FindStringSubmatch(line)
		if len(captured) != 2 {
			continue
		}
		value, ok := toPositiveInt(captured[1])
		if ok {
			matches = append(matches, value)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("[%s] Comなし", sectionName)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("[%s] Com複数", sectionName)
	}
	return &matches[0], nil
}

func IsSectionAutoOff(text string, sectionName string) bool {
	lines, _ := splitLines(text)
	target, found := findSectionRange(lines, sectionName)
	if !found {
		return false
	}
	return testCommentedSectionLine(lines[target.Start])
}

func SetSectionComValue(text string, sectionName string, comValue int) (
	PatchResult, error,
) {
	lines, newline := splitLines(text)
	target, found := findSectionRange(lines, sectionName)
	if !found {
		return PatchResult{}, fmt.Errorf("[%s] セクションが見つかりません", sectionName)
	}

	matches := make([]int, 0, 2)
	for idx := target.Start; idx <= target.End; idx++ {
		normalized := removeManagedCommentPrefix(lines[idx])
		trim := strings.TrimSpace(normalized)
		if isCommentLine(trim) {
			continue
		}
		if comPrefixPattern.MatchString(normalized) {
			matches = append(matches, idx)
		}
	}

	if len(matches) == 0 {
		return PatchResult{}, fmt.Errorf("[%s] の有効なCom行がありません", sectionName)
	}
	if len(matches) > 1 {
		return PatchResult{}, fmt.Errorf("[%s] の有効なCom行が複数です", sectionName)
	}

	idx := matches[0]
	original := lines[idx]
	normalized := removeAutoCommentPrefix(original)
	prefix := comPrefixPattern.FindString(normalized)
	replaced := prefix + fmt.Sprintf("%d", comValue)
	if testAutoOffLine(original) {
		replaced = AutoCommentPrefix + replaced
	}
	changed := original != replaced
	if changed {
		lines[idx] = replaced
	}

	return PatchResult{
		Changed: changed,
		Text:    joinLines(lines, newline, hasTrailingNewline(text)),
	}, nil
}

func SetIrsDevice2CommentState(text string, useDevice2 bool) (
	PatchResult, error,
) {
	lines, newline := splitLines(text)
	target, found := findSectionRange(lines, "DEVICE2")
	if !found {
		return PatchResult{}, fmt.Errorf("[DEVICE2] セクションが見つかりません")
	}

	changed := false
	for idx := target.Start; idx <= target.End; idx++ {
		original := lines[idx]
		replaced := original
		if useDevice2 {
			replaced = removeManagedCommentPrefix(original)
		}
		if !useDevice2 {
			replaced = AutoCommentPrefix + removeManagedCommentPrefix(original)
		}
		if replaced != original {
			lines[idx] = replaced
			changed = true
		}
	}

	return PatchResult{
		Changed: changed,
		Text:    joinLines(lines, newline, hasTrailingNewline(text)),
	}, nil
}

func UpdateIrsText(text string, device1Com int, useDevice2 bool, device2Com *int) (
	PatchResult, error,
) {
	step1, err := SetSectionComValue(text, "DEVICE1", device1Com)
	if err != nil {
		return PatchResult{}, err
	}
	step2, err := SetIrsDevice2CommentState(step1.Text, useDevice2)
	if err != nil {
		return PatchResult{}, err
	}
	changed := step1.Changed || step2.Changed
	if !useDevice2 {
		return PatchResult{Changed: changed, Text: step2.Text}, nil
	}
	if device2Com == nil {
		return PatchResult{}, fmt.Errorf("IRS DEVICE2 が選択されていますがCom未入力です")
	}
	step3, err := SetSectionComValue(step2.Text, "DEVICE2", *device2Com)
	if err != nil {
		return PatchResult{}, err
	}
	return PatchResult{
		Changed: changed || step3.Changed,
		Text:    step3.Text,
	}, nil
}

func removeAutoCommentPrefix(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, AutoCommentPrefix) {
		return line
	}
	pos := strings.Index(line, AutoCommentPrefix)
	if pos < 0 {
		return line
	}
	return line[:pos] + strings.TrimPrefix(trimmed, AutoCommentPrefix)
}

func removeManagedCommentPrefix(line string) string {
	step1 := removeAutoCommentPrefix(line)
	trimmed := strings.TrimLeft(step1, " \t")
	if !strings.HasPrefix(trimmed, ";") {
		return step1
	}
	prefixLen := len(step1) - len(trimmed)
	uncommented := strings.TrimLeft(strings.TrimPrefix(trimmed, ";"), " \t")
	return step1[:prefixLen] + uncommented
}

func testAutoOffLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, AutoCommentPrefix)
}

func testCommentedSectionLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, AutoCommentPrefix) {
		return true
	}
	return strings.HasPrefix(trimmed, ";")
}

func findSectionRange(lines []string, sectionName string) (sectionRange, bool) {
	header := "[" + sectionName + "]"
	start := -1
	for idx := range lines {
		normalized := strings.TrimSpace(removeManagedCommentPrefix(lines[idx]))
		if normalized == header {
			start = idx
			break
		}
	}
	if start < 0 {
		return sectionRange{}, false
	}

	end := len(lines) - 1
	for idx := start + 1; idx < len(lines); idx++ {
		normalized := strings.TrimSpace(removeManagedCommentPrefix(lines[idx]))
		if sectionHeaderPattern.MatchString(normalized) {
			end = idx - 1
			break
		}
	}
	return sectionRange{Start: start, End: end}, true
}

func splitLines(text string) ([]string, string) {
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1], newline
	}
	return lines, newline
}

func joinLines(lines []string, newline string, withTrailingNewline bool) string {
	body := strings.Join(lines, newline)
	if withTrailingNewline {
		return body + newline
	}
	return body
}

func hasTrailingNewline(text string) bool {
	return strings.HasSuffix(text, "\r\n") || strings.HasSuffix(text, "\n")
}

func isCommentLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#")
}

func toPositiveInt(raw string) (int, bool) {
	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		value = value*10 + int(ch-'0')
	}
	if value <= 0 {
		return 0, false
	}
	return value, true
}
