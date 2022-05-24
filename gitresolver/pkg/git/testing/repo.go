/*
 Copyright 2022 The Tekton Authors

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package testing

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CreateTestRepo is used to instantiate a local test repository with the desired commits.
func CreateTestRepo(t *testing.T, commits []CommitForRepo) (string, map[string][]string) {
	t.Helper()
	tempDir := t.TempDir()

	repo, err := git.PlainInit(tempDir, false)

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("getting test worktree: %v", err)
	}
	if worktree == nil {
		t.Fatal("test worktree not created")
	}

	startingFile := filepath.Join(tempDir, "README")
	if err := ioutil.WriteFile(startingFile, []byte("This is a test"), 0600); err != nil {
		t.Fatalf("couldn't write content to file %s: %v", startingFile, err)
	}

	_, err = worktree.Add("README")
	if err != nil {
		t.Fatalf("couldn't add file %s to git: %v", startingFile, err)
	}

	startingHash, err := worktree.Commit("adding file for test", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Someone",
			Email: "someone@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("couldn't perform commit for test: %v", err)
	}

	hashesByBranch := make(map[string][]string)

	// Iterate over the commits and add them.
	for _, cmt := range commits {
		branch := cmt.Branch
		if branch == "" {
			branch = plumbing.Master.Short()
		}

		// If we're given a branch, check out that branch.
		coOpts := &git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(branch),
		}

		if _, ok := hashesByBranch[branch]; !ok && branch != plumbing.Master.Short() {
			coOpts.Hash = startingHash
			coOpts.Create = true
		}

		if err := worktree.Checkout(coOpts); err != nil {
			t.Fatalf("couldn't do checkout of %s: %v", branch, err)
		}

		targetDir := filepath.Join(tempDir, cmt.Dir)
		fi, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetDir, 0700); err != nil {
				t.Fatalf("couldn't create directory %s in worktree: %v", targetDir, err)
			}
		} else if err != nil {
			t.Fatalf("checking if directory %s in worktree exists: %v", targetDir, err)
		}
		if fi != nil && !fi.IsDir() {
			t.Fatalf("%s already exists but is not a directory", targetDir)
		}

		outfile := filepath.Join(targetDir, cmt.Filename)
		if err := ioutil.WriteFile(outfile, []byte(cmt.Content), 0600); err != nil {
			t.Fatalf("couldn't write content to file %s: %v", outfile, err)
		}

		_, err = worktree.Add(filepath.Join(cmt.Dir, cmt.Filename))
		if err != nil {
			t.Fatalf("couldn't add file %s to git: %v", outfile, err)
		}

		hash, err := worktree.Commit("adding file for test", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Someone",
				Email: "someone@example.com",
				When:  time.Now(),
			},
		})
		if err != nil {
			t.Fatalf("couldn't perform commit for test: %v", err)
		}

		if _, ok := hashesByBranch[branch]; !ok {
			hashesByBranch[branch] = []string{hash.String()}
		} else {
			hashesByBranch[branch] = append(hashesByBranch[branch], hash.String())
		}
	}

	return tempDir, hashesByBranch
}

// CommitForRepo provides the directory, filename, content and branch for a test commit.
type CommitForRepo struct {
	Dir      string
	Filename string
	Content  string
	Branch   string
}

// WithTemporaryGitConfig resets the .gitconfig for the duration of the test.
func WithTemporaryGitConfig(t *testing.T) func() {
	gitConfigDir := t.TempDir()
	key := "GIT_CONFIG_GLOBAL"
	t.Helper()
	oldValue, envVarExists := os.LookupEnv(key)
	if err := os.Setenv(key, filepath.Join(gitConfigDir, "config")); err != nil {
		t.Fatal(err)
	}
	clean := func() {
		t.Helper()
		if !envVarExists {
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.Setenv(key, oldValue); err != nil {
			t.Fatal(err)
		}
	}
	return clean
}
