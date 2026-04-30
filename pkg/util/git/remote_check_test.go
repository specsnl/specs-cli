package git

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
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
	result := resolveStatus(refs, hashA, "main")
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
	result := resolveStatus(refs, hashA, "main")
	if result.IsUpToDate {
		t.Error("expected IsUpToDate = false when branch hash differs from local HEAD")
	}
	if result.ErrorKind != CheckErrorNone {
		t.Errorf("expected no error, got %q", result.ErrorKind)
	}
}

func TestResolveStatus_TagAlreadyLatest(t *testing.T) {
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewTagReferenceName("v1.0.0"), hashA),
	}
	result := resolveStatus(refs, hashA, "v1.0.0")
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
	result := resolveStatus(refs, hashA, "v1.0.0")
	if result.IsUpToDate {
		t.Error("expected IsUpToDate = false when newer tag exists")
	}
	if result.LatestVersion != "v2.0.0" {
		t.Errorf("LatestVersion: got %q, want %q", result.LatestVersion, "v2.0.0")
	}
}

func TestResolveStatus_RefNotFound(t *testing.T) {
	result := resolveStatus(nil, plumbing.ZeroHash, "main")
	if result.ErrorKind != CheckErrorNotFound {
		t.Errorf("expected CheckErrorNotFound, got %q", result.ErrorKind)
	}
}

func TestLatestSemverTag_NewerExists(t *testing.T) {
	tags := map[string]struct{}{
		"v1.0.0": {},
		"v1.1.0": {},
		"v2.0.0": {},
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
