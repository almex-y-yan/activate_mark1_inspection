package main

import (
	"slices"
	"testing"
)

func TestDefaultInspectionStartRequestSelectsAllTargets(t *testing.T) {
	request := defaultInspectionStartRequest()

	if !request.Card.Selected {
		t.Fatal("expected Card to be selected by default")
	}
	if !request.IRS.Selected {
		t.Fatal("expected IRS to be selected by default")
	}
	if !request.NM43.Selected {
		t.Fatal("expected NM43 to be selected by default")
	}
}

func TestBuildInspectionStartServicesSkipsOnlyUnselectedReaders(t *testing.T) {
	request := defaultInspectionStartRequest()
	request.Card.Selected = false
	request.IRS.Selected = false

	services, logs := buildInspectionStartServices(request)

	if slices.Contains(services, "almdevcd7") {
		t.Fatal("did not expect Card service to be started")
	}
	if slices.Contains(services, "almdevic2") {
		t.Fatal("did not expect IRS service to be started")
	}
	if !slices.Contains(services, "almdevic5") {
		t.Fatal("expected NM43 service to remain in the start list")
	}
	if !slices.Contains(logs, "開始スキップ: almdevcd7 (Card 未選択)") {
		t.Fatal("expected Card skip log")
	}
	if !slices.Contains(logs, "開始スキップ: almdevic2 (IRS 未選択)") {
		t.Fatal("expected IRS skip log")
	}
}
