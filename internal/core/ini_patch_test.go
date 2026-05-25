package core

import (
	"strings"
	"testing"
)

func TestSetSectionComValue(t *testing.T) {
	src := "[DEVICE1]\nCom = 2\nBps = 9600\n"
	out, err := SetSectionComValue(src, "DEVICE1", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Changed {
		t.Fatal("expected changed=true")
	}
	if !strings.Contains(out.Text, "Com = 7") {
		t.Fatalf("unexpected text: %s", out.Text)
	}
}

func TestUpdateIrsText(t *testing.T) {
	src := strings.Join([]string{
		"[DEVICE1]",
		"Com=1",
		"[DEVICE2]",
		"Com=2",
		"",
	}, "\n")
	value := 9
	out, err := UpdateIrsText(src, 3, true, &value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Changed {
		t.Fatal("expected changed=true")
	}
	if !strings.Contains(out.Text, "Com=3") {
		t.Fatal("device1 was not updated")
	}
	if !strings.Contains(out.Text, "Com=9") {
		t.Fatal("device2 was not updated")
	}
}

func TestSetIrsDevice2CommentState(t *testing.T) {
	src := strings.Join([]string{
		"[DEVICE2]",
		"Com=2",
		"Name=X",
		"",
	}, "\n")
	out, err := SetIrsDevice2CommentState(src, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.Text, AutoCommentPrefix+"[DEVICE2]") {
		t.Fatal("expected commented DEVICE2 section")
	}
}

func TestGetSectionComValueWithSemicolonCommentedSection(t *testing.T) {
	src := strings.Join([]string{
		";[DEVICE2]",
		";Com=8",
		";Name=X",
		"",
	}, "\n")
	value, err := GetSectionComValue(src, "DEVICE2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value == nil || *value != 8 {
		t.Fatalf("unexpected value: %v", value)
	}
}

func TestSetIrsDevice2CommentStateUncommentSemicolonSection(t *testing.T) {
	src := strings.Join([]string{
		";[DEVICE2]",
		";Com=2",
		";Name=X",
		"",
	}, "\n")
	out, err := SetIrsDevice2CommentState(src, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.Text, ";[DEVICE2]") {
		t.Fatal("expected DEVICE2 section uncommented")
	}
	if !strings.Contains(out.Text, "[DEVICE2]") {
		t.Fatal("expected DEVICE2 header")
	}
}

func TestIsSectionAutoOffWithSemicolonCommentedSection(t *testing.T) {
	src := strings.Join([]string{
		";[DEVICE2]",
		";Com=2",
		"",
	}, "\n")
	if !IsSectionAutoOff(src, "DEVICE2") {
		t.Fatal("expected DEVICE2 as off")
	}
}
