package lint_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"archdoc.dev/internal/lint"
)

// releaseTag is the tag the release workflow is publishing, and empty on every
// other run.
const releaseTag = "ARCHDOC_RELEASE_TAG"

// compareVersions orders two tags by semantic version precedence: by major,
// minor and patch as numbers, a pre-release before the release it precedes,
// and two pre-releases by their dot-separated identifiers, numbers as numbers
// and before any word.
func compareVersions(a, b string) int {
	coreA, preA, _ := strings.Cut(strings.TrimPrefix(a, "v"), "-")
	coreB, preB, _ := strings.Cut(strings.TrimPrefix(b, "v"), "-")
	if c := compareIdentifiers(coreA, coreB); c != 0 {
		return c
	}
	switch {
	case preA == preB:
		return 0
	case preA == "":
		return 1
	case preB == "":
		return -1
	}
	return compareIdentifiers(preA, preB)
}

// compareIdentifiers compares dot-separated identifiers in turn, a shorter
// list first where one is a prefix of the other.
func compareIdentifiers(a, b string) int {
	partsA, partsB := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(partsA), len(partsB)) {
		x, errX := strconv.Atoi(partsA[i])
		y, errY := strconv.Atoi(partsB[i])
		switch {
		case errX == nil && errY == nil && x != y:
			if x < y {
				return -1
			}
			return 1
		case errX == nil && errY != nil:
			return -1
		case errX != nil && errY == nil:
			return 1
		case errX != nil && errY != nil && partsA[i] != partsB[i]:
			return strings.Compare(partsA[i], partsB[i])
		}
	}
	return len(partsA) - len(partsB)
}

func TestVersionsCompareByPrecedence(t *testing.T) {
	// In ascending order, each pair adjacent in semver's own example.
	ordered := []string{
		"v0.1.0-alpha.1", "v0.1.0-alpha.2", "v0.1.0-alpha.10", "v0.1.0-beta", "v0.1.0",
		"v0.1.1", "v0.5.0", "v0.6.0-alpha", "v0.6.0-alpha.1", "v0.6.0-alpha.beta", "v0.6.0", "v0.10.0", "v1.0.0",
	}
	for i := range ordered {
		for j := range ordered {
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got := compareVersions(ordered[i], ordered[j]); max(-1, min(1, got)) != want {
				t.Errorf("compareVersions(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}

func TestNoRuleNamesALaterRelease(t *testing.T) {
	tag := os.Getenv(releaseTag)
	if tag == "" {
		t.Skipf("%s is set only by the release workflow", releaseTag)
	}
	for _, rule := range lint.Rules {
		if compareVersions(rule.Since, tag) > 0 {
			t.Errorf("%s gives since %s, which is later than the release %s", rule.Code, rule.Since, tag)
		}
		for _, c := range rule.Changes {
			if compareVersions(c.Release, tag) > 0 {
				t.Errorf("%s records a change in %s, which is later than the release %s", rule.Code, c.Release, tag)
			}
		}
	}
}

func TestTheReleaseWorkflowNamesItsTag(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), releaseTag+": ${{ github.ref_name }}") {
		t.Errorf("release.yml does not set %s, so TestNoRuleNamesALaterRelease never runs", releaseTag)
	}
}
