package git

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func TestClassifyRemoteError_Network(t *testing.T) {
	err := &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")}

	got := classifyRemoteError(err)
	if got != CheckErrorNetwork {
		t.Errorf("classifyRemoteError(&net.OpError): got %q, want %q", got, CheckErrorNetwork)
	}
}

func TestClassifyRemoteError_Auth_AuthenticationRequired(t *testing.T) {
	got := classifyRemoteError(transport.ErrAuthenticationRequired)
	if got != CheckErrorAuth {
		t.Errorf("classifyRemoteError(ErrAuthenticationRequired): got %q, want %q", got, CheckErrorAuth)
	}
}

func TestClassifyRemoteError_Auth_AuthorizationFailed(t *testing.T) {
	got := classifyRemoteError(transport.ErrAuthorizationFailed)
	if got != CheckErrorAuth {
		t.Errorf("classifyRemoteError(ErrAuthorizationFailed): got %q, want %q", got, CheckErrorAuth)
	}
}

func TestClassifyRemoteError_NotFound(t *testing.T) {
	got := classifyRemoteError(transport.ErrRepositoryNotFound)
	if got != CheckErrorNotFound {
		t.Errorf("classifyRemoteError(ErrRepositoryNotFound): got %q, want %q", got, CheckErrorNotFound)
	}
}

func TestClassifyRemoteError_Unknown(t *testing.T) {
	got := classifyRemoteError(errors.New("some unexpected error"))
	if got != CheckErrorUnknown {
		t.Errorf("classifyRemoteError(unknown): got %q, want %q", got, CheckErrorUnknown)
	}
}

var (
	hashA = plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	hashB = plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
)

func TestResolveStatus_BranchUpToDate(t *testing.T) {
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), hashA),
	}

	result := resolveStatus(refs, hashA, "main", "")
	if !result.IsUpToDate {
		t.Error("expected IsUpToDate = true when branch hash matches local HEAD")
	}

	if result.ErrorKind != CheckErrorNone {
		t.Errorf("expected no error, got %q", result.ErrorKind)
	}
}

func TestResolveStatus_BranchBehind(t *testing.T) {
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), hashB),
	}

	result := resolveStatus(refs, hashA, "main", "")
	if result.IsUpToDate {
		t.Error("expected IsUpToDate = false when branch hash differs from local HEAD")
	}

	if result.ErrorKind != CheckErrorNone {
		t.Errorf("expected no error, got %q", result.ErrorKind)
	}
}

// TestResolveStatus_BranchOnSemverTagNotOutdatedByLowerTag reproduces issue #83:
// a branch-tracked template checked out on 1.1.0 must not be reported as outdated
// when the branch advances to a commit tagged with a lower version (1.0.1).
func TestResolveStatus_BranchOnSemverTagNotOutdatedByLowerTag(t *testing.T) {
	refs := []*plumbing.Reference{
		// Branch tip moved to the 1.0.1 commit (hashB), local HEAD is still on 1.1.0 (hashA).
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), hashB),
		plumbing.NewHashReference(plumbing.NewTagReferenceName("1.1.0"), hashA),
		plumbing.NewHashReference(plumbing.NewTagReferenceName("1.0.1"), hashB),
	}

	result := resolveStatus(refs, hashA, "main", "1.1.0")
	if !result.IsUpToDate {
		t.Error("expected IsUpToDate = true: 1.0.1 is not a semver upgrade over 1.1.0")
	}

	if result.LatestVersion != "" {
		t.Errorf("expected empty LatestVersion, got %q", result.LatestVersion)
	}
}

// TestResolveStatus_BranchOnSemverTagUpgradesToHigherTag verifies the branch path
// still surfaces a genuinely higher semver tag as an update.
func TestResolveStatus_BranchOnSemverTagUpgradesToHigherTag(t *testing.T) {
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), hashB),
		plumbing.NewHashReference(plumbing.NewTagReferenceName("1.1.0"), hashA),
		plumbing.NewHashReference(plumbing.NewTagReferenceName("1.2.0"), hashB),
	}

	result := resolveStatus(refs, hashA, "main", "1.1.0")
	if result.IsUpToDate {
		t.Error("expected IsUpToDate = false when a higher semver tag exists")
	}

	if result.LatestVersion != "1.2.0" {
		t.Errorf("LatestVersion: got %q, want %q", result.LatestVersion, "1.2.0")
	}
}

// TestResolveStatus_BranchNonSemverFallsBackToCommit verifies that a rolling branch
// whose checkout is not on a released version still tracks the branch tip by commit.
func TestResolveStatus_BranchNonSemverFallsBackToCommit(t *testing.T) {
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), hashB),
	}

	result := resolveStatus(refs, hashA, "main", "")
	if result.IsUpToDate {
		t.Error("expected IsUpToDate = false: non-semver checkout falls back to commit comparison")
	}
}

func TestResolveStatus_TagAlreadyLatest(t *testing.T) {
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewTagReferenceName("v1.0.0"), hashA),
	}

	result := resolveStatus(refs, hashA, "v1.0.0", "v1.0.0")
	if !result.IsUpToDate {
		t.Error("expected IsUpToDate = true when on latest semver tag")
	}

	if result.LatestVersion != "" {
		t.Errorf("expected empty LatestVersion, got %q", result.LatestVersion)
	}
}

func TestResolveStatus_TagNewerExists(t *testing.T) {
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewTagReferenceName("v1.0.0"), hashA),
		plumbing.NewHashReference(plumbing.NewTagReferenceName("v2.0.0"), hashB),
	}

	result := resolveStatus(refs, hashA, "v1.0.0", "v1.0.0")
	if result.IsUpToDate {
		t.Error("expected IsUpToDate = false when newer tag exists")
	}

	if result.LatestVersion != "v2.0.0" {
		t.Errorf("LatestVersion: got %q, want %q", result.LatestVersion, "v2.0.0")
	}
}

func TestResolveStatus_RefNotFound(t *testing.T) {
	result := resolveStatus(nil, plumbing.ZeroHash, "main", "")
	if result.ErrorKind != CheckErrorNotFound {
		t.Errorf("expected CheckErrorNotFound, got %q", result.ErrorKind)
	}
}

// commitFile stages a file and commits it, returning the new commit hash.
func commitFile(t *testing.T, repo *gogit.Repository, dir, name, msg string) plumbing.Hash {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if _, err := wt.Add(name); err != nil {
		t.Fatalf("add: %v", err)
	}

	sig := &object.Signature{Name: "T", Email: "t@example.com", When: time.Unix(0, 0).UTC()}

	h, err := wt.Commit(msg, &gogit.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	return h
}

func TestSemverTagAtCommit(t *testing.T) {
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	tagged := commitFile(t, repo, dir, "a", "first")
	// A lightweight and an (higher) annotated tag both on the same commit.
	if _, err := repo.CreateTag("1.1.0", tagged, nil); err != nil {
		t.Fatalf("lightweight tag: %v", err)
	}

	sig := &object.Signature{Name: "T", Email: "t@example.com", When: time.Unix(0, 0).UTC()}
	if _, err := repo.CreateTag("v1.2.0", tagged, &gogit.CreateTagOptions{Message: "release", Tagger: sig}); err != nil {
		t.Fatalf("annotated tag: %v", err)
	}

	untagged := commitFile(t, repo, dir, "b", "second")

	// Highest semver tag at the commit wins, and annotated tags are dereferenced.
	if got := semverTagAtCommit(repo, tagged); got != "v1.2.0" {
		t.Errorf("semverTagAtCommit(tagged): got %q, want %q", got, "v1.2.0")
	}
	// A commit with no tag on it yields no version.
	if got := semverTagAtCommit(repo, untagged); got != "" {
		t.Errorf("semverTagAtCommit(untagged): got %q, want empty string", got)
	}
}

func TestLatestSemverTag_NewerExists(t *testing.T) {
	tags := map[string]struct{}{
		"v1.0.0":     {},
		"v1.1.0":     {},
		"v2.0.0":     {},
		"not-semver": {},
	}

	got := latestSemverTag(tags, "v1.1.0")
	if got != "v2.0.0" {
		t.Errorf("latestSemverTag: got %q, want %q", got, "v2.0.0")
	}
}

func TestLatestSemverTag_AlreadyLatest(t *testing.T) {
	tags := map[string]struct{}{
		"v1.0.0": {},
		"v1.1.0": {},
	}

	got := latestSemverTag(tags, "v1.1.0")
	if got != "" {
		t.Errorf("latestSemverTag: got %q, want empty string (already latest)", got)
	}
}

func TestLatestSemverTag_InvalidCurrent(t *testing.T) {
	tags := map[string]struct{}{"v1.0.0": {}}

	got := latestSemverTag(tags, "not-a-version")
	if got != "" {
		t.Errorf("latestSemverTag: got %q, want empty string for invalid current", got)
	}
}
