package git

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-cmp/cmp"
	gittesting "github.com/tektoncd/resolution/gitresolver/pkg/git/testing"
	resolutioncommon "github.com/tektoncd/resolution/pkg/common"
	"github.com/tektoncd/resolution/pkg/resolver/framework"
	"github.com/tektoncd/resolution/test/diff"
)

func TestGetSelector(t *testing.T) {
	resolver := Resolver{}
	sel := resolver.GetSelector(context.Background())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != LabelValueGitResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	resolver := Resolver{}

	paramsWithCommit := map[string]string{
		URLParam:    "foo",
		PathParam:   "bar",
		CommitParam: "baz",
	}
	if err := resolver.ValidateParams(context.Background(), paramsWithCommit); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}

	paramsWithBranch := map[string]string{
		URLParam:    "foo",
		PathParam:   "bar",
		BranchParam: "baz",
	}
	if err := resolver.ValidateParams(context.Background(), paramsWithBranch); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateParamsMissing(t *testing.T) {
	resolver := Resolver{}

	var err error

	paramsMissingURL := map[string]string{
		PathParam:   "bar",
		CommitParam: "baz",
	}
	err = resolver.ValidateParams(context.Background(), paramsMissingURL)
	if err == nil {
		t.Fatalf("expected missing url err")
	}

	paramsMissingPath := map[string]string{
		URLParam:    "foo",
		BranchParam: "baz",
	}
	err = resolver.ValidateParams(context.Background(), paramsMissingPath)
	if err == nil {
		t.Fatalf("expected missing path err")
	}
}

func TestValidateParamsConflictingGitRef(t *testing.T) {
	resolver := Resolver{}
	params := map[string]string{
		URLParam:    "foo",
		PathParam:   "bar",
		CommitParam: "baz",
		BranchParam: "quux",
	}
	err := resolver.ValidateParams(context.Background(), params)
	if err == nil {
		t.Fatalf("expected err due to conflicting commit and branch params")
	}
}

func TestGetResolutionTimeoutDefault(t *testing.T) {
	resolver := Resolver{}
	defaultTimeout := 30 * time.Minute
	timeout := resolver.GetResolutionTimeout(context.Background(), defaultTimeout)
	if timeout != defaultTimeout {
		t.Fatalf("expected default timeout to be returned")
	}
}

func TestGetResolutionTimeoutCustom(t *testing.T) {
	resolver := Resolver{}
	defaultTimeout := 30 * time.Minute
	configTimeout := 5 * time.Second
	config := map[string]string{
		ConfigFieldTimeout: configTimeout.String(),
	}
	ctx := framework.InjectResolverConfigToContext(context.Background(), config)
	timeout := resolver.GetResolutionTimeout(ctx, defaultTimeout)
	if timeout != configTimeout {
		t.Fatalf("expected timeout from config to be returned")
	}
}

func TestResolve(t *testing.T) {
	gittesting.WithTemporaryGitConfig(t)

	testCases := []struct {
		name            string
		commits         []gittesting.CommitForRepo
		branch          string
		useNthCommit    int
		specificCommit  string
		path            string
		filename        string
		expectedContent []byte
		expectedErr     error
	}{
		{
			name: "single commit",
			commits: []gittesting.CommitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			path:            "foo/bar",
			filename:        "somefile",
			expectedContent: []byte("some content"),
		}, {
			name: "with branch",
			commits: []gittesting.CommitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
				Branch:   "other-branch",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "wrong content",
			}},
			branch:          "other-branch",
			path:            "foo/bar",
			filename:        "somefile",
			expectedContent: []byte("some content"),
		}, {
			name: "earlier specific commit",
			commits: []gittesting.CommitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}, {
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "different content",
			}},
			path:            "foo/bar",
			filename:        "somefile",
			useNthCommit:    1,
			expectedContent: []byte("different content"),
		}, {
			name: "file does not exist",
			commits: []gittesting.CommitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			path:        "foo/bar",
			filename:    "some other file",
			expectedErr: errors.New(`error opening file "foo/bar/some other file": file does not exist`),
		}, {
			name: "branch does not exist",
			commits: []gittesting.CommitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			branch:      "does-not-exist",
			path:        "foo/bar",
			filename:    "some other file",
			expectedErr: errors.New(`clone error: couldn't find remote ref "refs/heads/does-not-exist"`),
		}, {
			name: "commit does not exist",
			commits: []gittesting.CommitForRepo{{
				Dir:      "foo/bar",
				Filename: "somefile",
				Content:  "some content",
			}},
			specificCommit: "does-not-exist",
			path:           "foo/bar",
			filename:       "some other file",
			expectedErr:    errors.New("checkout error: object not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoPath, commits := gittesting.CreateTestRepo(t, tc.commits)

			resolver := &Resolver{}

			params := map[string]string{
				URLParam:  repoPath,
				PathParam: filepath.Join(tc.path, tc.filename),
			}

			if tc.branch != "" {
				params[BranchParam] = tc.branch
			}

			if tc.useNthCommit > 0 {
				params[CommitParam] = commits[plumbing.Master.Short()][tc.useNthCommit]
			} else if tc.specificCommit != "" {
				params[CommitParam] = hex.EncodeToString([]byte(tc.specificCommit))
			}

			output, err := resolver.Resolve(context.Background(), params)
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected err '%v' but didn't get one", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("expected err '%v' but got '%v'", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error resolving: %v", err)
				}

				expectedResource := &ResolvedGitResource{
					Content: tc.expectedContent,
				}
				switch {
				case tc.useNthCommit > 0:
					expectedResource.Commit = commits[plumbing.Master.Short()][tc.useNthCommit]
				case tc.branch != "":
					expectedResource.Commit = commits[tc.branch][len(commits[tc.branch])-1]
				default:
					expectedResource.Commit = commits[plumbing.Master.Short()][len(commits[plumbing.Master.Short()])-1]
				}

				if d := cmp.Diff(expectedResource, output); d != "" {
					t.Errorf("unexpected resource from Resolve: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}
