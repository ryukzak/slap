package util

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_AbsoluteLinkOpensInNewTab(t *testing.T) {
	got := string(RenderMarkdown("see [docs](https://example.com/page)"))
	if !strings.Contains(got, `href="https://example.com/page"`) {
		t.Fatalf("expected absolute href, got: %s", got)
	}
	if !strings.Contains(got, `target="_blank"`) {
		t.Errorf("expected target=\"_blank\" on absolute link, got: %s", got)
	}
	if !strings.Contains(got, `rel=`) || !strings.Contains(got, "noopener") {
		t.Errorf("expected rel with noopener on absolute link, got: %s", got)
	}
}

func TestRenderMarkdown_RelativeLinkStaysInSameTab(t *testing.T) {
	got := string(RenderMarkdown("see [profile](/user/42)"))
	if !strings.Contains(got, `href="/user/42"`) {
		t.Fatalf("expected relative href, got: %s", got)
	}
	if strings.Contains(got, `target="_blank"`) {
		t.Errorf("did not expect target=\"_blank\" on relative link, got: %s", got)
	}
}

func TestRenderMarkdown_ImagesAreStripped(t *testing.T) {
	got := string(RenderMarkdown("see my cool image ![cool image](/logout)"))
	if strings.Contains(got, `src="/logout"`) {
		t.Fatalf("expected image removed, got: %s", got)
	}
}

func TestRenderMarkdown_HtmlImagesAreStripped(t *testing.T) {
	got := string(RenderMarkdown("see my cool image <img alt=\"cool image\" src=\"/logout\"</img>"))
	if strings.Contains(got, `src="/logout"`) {
		t.Fatalf("expected image removed, got: %s", got)
	}
}
