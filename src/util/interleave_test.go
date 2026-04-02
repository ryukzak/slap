package util

import (
	"strings"
	"testing"
)

type item struct {
	group string
	id    string
}

func byGroup(i item) string { return i.group }

func keys(items []item) string {
	parts := make([]string, len(items))
	for i, v := range items {
		parts[i] = v.group
	}
	return strings.Join(parts, ",")
}

func TestInterleaveByKey_Empty(t *testing.T) {
	result := InterleaveByKey([]item{}, byGroup)
	if len(result) != 0 {
		t.Fatalf("expected empty, got %d", len(result))
	}
}

func TestInterleaveByKey_Nil(t *testing.T) {
	result := InterleaveByKey(nil, byGroup)
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestInterleaveByKey_SingleGroup(t *testing.T) {
	input := []item{{"a", "1"}, {"a", "2"}, {"a", "3"}}
	result := InterleaveByKey(input, byGroup)
	got := keys(result)
	if got != "a,a,a" {
		t.Fatalf("expected a,a,a, got %s", got)
	}
	if result[0].id != "1" || result[1].id != "2" || result[2].id != "3" {
		t.Fatalf("within-group order not preserved")
	}
}

func TestInterleaveByKey_TwoEqualGroups(t *testing.T) {
	input := []item{{"a", "1"}, {"a", "2"}, {"b", "3"}, {"b", "4"}}
	result := InterleaveByKey(input, byGroup)
	got := keys(result)
	if got != "a,b,a,b" {
		t.Fatalf("expected a,b,a,b, got %s", got)
	}
}

func TestInterleaveByKey_UnequalGroups(t *testing.T) {
	input := []item{{"a", "1"}, {"a", "2"}, {"a", "3"}, {"b", "4"}}
	result := InterleaveByKey(input, byGroup)
	got := keys(result)
	if got != "a,b,a,a" {
		t.Fatalf("expected a,b,a,a, got %s", got)
	}
}

func TestInterleaveByKey_ThreeGroups(t *testing.T) {
	input := []item{{"a", "1"}, {"a", "2"}, {"b", "3"}, {"b", "4"}, {"c", "5"}}
	result := InterleaveByKey(input, byGroup)
	got := keys(result)
	if got != "a,b,c,a,b" {
		t.Fatalf("expected a,b,c,a,b, got %s", got)
	}
}

func TestInterleaveByKey_WithinGroupOrderPreserved(t *testing.T) {
	input := []item{{"a", "1"}, {"a", "2"}, {"b", "3"}, {"b", "4"}}
	result := InterleaveByKey(input, byGroup)
	if result[0].id != "1" || result[1].id != "3" || result[2].id != "2" || result[3].id != "4" {
		t.Fatalf("expected interleaved with preserved group order, got %v", result)
	}
}
