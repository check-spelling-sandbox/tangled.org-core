package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Helper struct {
	t       *testing.T
	tempDir string
	repo    *GitRepo
}

func helper(t *testing.T) *Helper {
	tempDir, err := os.MkdirTemp("", "git-merge-test-*")
	require.NoError(t, err)

	return &Helper{
		t:       t,
		tempDir: tempDir,
	}
}

func (h *Helper) cleanup() {
	if h.tempDir != "" {
		os.RemoveAll(h.tempDir)
	}
}

// initRepo initializes a git repository with an initial commit
func (h *Helper) initRepo() *GitRepo {
	repoPath := filepath.Join(h.tempDir, "test-repo")

	// initialize repository
	r, err := git.PlainInit(repoPath, false)
	require.NoError(h.t, err)

	// configure git user
	cfg, err := r.Config()
	require.NoError(h.t, err)
	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	err = r.SetConfig(cfg)
	require.NoError(h.t, err)

	// create initial commit with a file
	w, err := r.Worktree()
	require.NoError(h.t, err)

	// create initial file
	initialFile := filepath.Join(repoPath, "README.md")
	err = os.WriteFile(initialFile, []byte("# Test Repository\n\nInitial content.\n"), 0644)
	require.NoError(h.t, err)

	_, err = w.Add("README.md")
	require.NoError(h.t, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(h.t, err)

	gitRepo, err := PlainOpen(repoPath)
	require.NoError(h.t, err)

	h.repo = gitRepo
	return gitRepo
}

// addFile creates a file in the repository
func (h *Helper) addFile(filename, content string) {
	filePath := filepath.Join(h.repo.path, filename)
	dir := filepath.Dir(filePath)

	err := os.MkdirAll(dir, 0755)
	require.NoError(h.t, err)

	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(h.t, err)
}

// commitFile adds and commits a file
func (h *Helper) commitFile(filename, content, message string) plumbing.Hash {
	h.addFile(filename, content)

	w, err := h.repo.r.Worktree()
	require.NoError(h.t, err)

	_, err = w.Add(filename)
	require.NoError(h.t, err)

	hash, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(h.t, err)

	return hash
}

// readFile reads a file from the repository
func (h *Helper) readFile(filename string) string {
	content, err := os.ReadFile(filepath.Join(h.repo.path, filename))
	require.NoError(h.t, err)
	return string(content)
}

// fileExists checks if a file exists in the repository
func (h *Helper) fileExists(filename string) bool {
	_, err := os.Stat(filepath.Join(h.repo.path, filename))
	return err == nil
}

func TestApplyPatch_Success(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// modify README.md
	patch := `diff --git a/README.md b/README.md
index 1234567..abcdefg 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,3 @@
 # Test Repository

-Initial content.
+Modified content.
`

	patchFile, err := createTemp(patch)
	require.NoError(t, err)
	defer os.Remove(patchFile)

	opts := MergeOptions{
		CommitMessage:  "Apply test patch",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	err = repo.applyPatch(patch, patchFile, opts)
	assert.NoError(t, err)

	// verify the file was modified
	content := h.readFile("README.md")
	assert.Contains(t, content, "Modified content.")
}

func TestApplyPatch_AddNewFile(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// add a new file
	patch := `diff --git a/newfile.txt b/newfile.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1 @@
+hello
`

	patchFile, err := createTemp(patch)
	require.NoError(t, err)
	defer os.Remove(patchFile)

	opts := MergeOptions{
		CommitMessage:  "Add new file",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	err = repo.applyPatch(patch, patchFile, opts)
	assert.NoError(t, err)

	assert.True(t, h.fileExists("newfile.txt"))
	content := h.readFile("newfile.txt")
	assert.Equal(t, "hello\n", content)
}

func TestApplyPatch_DeleteFile(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// add a file
	h.commitFile("deleteme.txt", "content to delete\n", "Add file to delete")

	// delete the file
	patch := `diff --git a/deleteme.txt b/deleteme.txt
deleted file mode 100644
index 1234567..0000000
--- a/deleteme.txt
+++ /dev/null
@@ -1 +0,0 @@
-content to delete
`

	patchFile, err := createTemp(patch)
	require.NoError(t, err)
	defer os.Remove(patchFile)

	opts := MergeOptions{
		CommitMessage:  "Delete file",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	err = repo.applyPatch(patch, patchFile, opts)
	assert.NoError(t, err)

	assert.False(t, h.fileExists("deleteme.txt"))
}

func TestApplyPatch_WithAuthor(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	patch := `diff --git a/README.md b/README.md
index 1234567..abcdefg 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,4 @@
 # Test Repository

 Initial content.
+New line.
`

	patchFile, err := createTemp(patch)
	require.NoError(t, err)
	defer os.Remove(patchFile)

	opts := MergeOptions{
		CommitMessage:  "Patch with author",
		AuthorName:     "Patch Author",
		AuthorEmail:    "author@example.com",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	err = repo.applyPatch(patch, patchFile, opts)
	assert.NoError(t, err)

	head, err := repo.r.Head()
	require.NoError(t, err)

	commit, err := repo.r.CommitObject(head.Hash())
	require.NoError(t, err)

	assert.Equal(t, "Patch Author", commit.Author.Name)
	assert.Equal(t, "author@example.com", commit.Author.Email)
}

func TestApplyPatch_MissingFile(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// patch that modifies a nonexistent file
	patch := `diff --git a/nonexistent.txt b/nonexistent.txt
index 1234567..abcdefg 100644
--- a/nonexistent.txt
+++ b/nonexistent.txt
@@ -1 +1 @@
-old content
+new content
`

	patchFile, err := createTemp(patch)
	require.NoError(t, err)
	defer os.Remove(patchFile)

	opts := MergeOptions{
		CommitMessage:  "Should fail",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	err = repo.applyPatch(patch, patchFile, opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "patch application failed")
}

func TestApplyPatch_Conflict(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// modify the file to create a conflict
	h.commitFile("README.md", "# Test Repository\n\nDifferent content.\n", "Modify README")

	// patch that expects different content
	patch := `diff --git a/README.md b/README.md
index 1234567..abcdefg 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,3 @@
 # Test Repository

-Initial content.
+Modified content.
`

	patchFile, err := createTemp(patch)
	require.NoError(t, err)
	defer os.Remove(patchFile)

	opts := MergeOptions{
		CommitMessage:  "Should conflict",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	err = repo.applyPatch(patch, patchFile, opts)
	assert.Error(t, err)
}

func TestApplyPatch_MissingDirectory(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// patch that adds a file in a nonexistent directory
	patch := `diff --git a/subdir/newfile.txt b/subdir/newfile.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/subdir/newfile.txt
@@ -0,0 +1 @@
+content
`

	patchFile, err := createTemp(patch)
	require.NoError(t, err)
	defer os.Remove(patchFile)

	opts := MergeOptions{
		CommitMessage:  "Add file in subdir",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	// git apply should create the directory automatically
	err = repo.applyPatch(patch, patchFile, opts)
	assert.NoError(t, err)

	// Verify the file and directory were created
	assert.True(t, h.fileExists("subdir/newfile.txt"))
}

func TestApplyMailbox_Single(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// format-patch mailbox format
	patch := `From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001
From: Patch Author <author@example.com>
Date: Mon, 1 Jan 2024 12:00:00 +0000
Subject: [PATCH] Add new feature

This is a test patch.
---
 newfile.txt | 1 +
 1 file changed, 1 insertion(+)
 create mode 100644 newfile.txt

diff --git a/newfile.txt b/newfile.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1 @@
+hello
--
2.40.0
`

	err := repo.applyMailbox(patch)
	assert.NoError(t, err)

	assert.True(t, h.fileExists("newfile.txt"))
	content := h.readFile("newfile.txt")
	assert.Equal(t, "hello\n", content)
}

func TestApplyMailbox_Multiple(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	// multiple patches in mailbox format
	patch := `From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001
From: Patch Author <author@example.com>
Date: Mon, 1 Jan 2024 12:00:00 +0000
Subject: [PATCH 1/2] Add first file

---
 file1.txt | 1 +
 1 file changed, 1 insertion(+)
 create mode 100644 file1.txt

diff --git a/file1.txt b/file1.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/file1.txt
@@ -0,0 +1 @@
+first
--
2.40.0

From 1111111111111111111111111111111111111111 Mon Sep 17 00:00:00 2001
From: Patch Author <author@example.com>
Date: Mon, 1 Jan 2024 12:01:00 +0000
Subject: [PATCH 2/2] Add second file

---
 file2.txt | 1 +
 1 file changed, 1 insertion(+)
 create mode 100644 file2.txt

diff --git a/file2.txt b/file2.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/file2.txt
@@ -0,0 +1 @@
+second
--
2.40.0
`

	err := repo.applyMailbox(patch)
	assert.NoError(t, err)

	assert.True(t, h.fileExists("file1.txt"))
	assert.True(t, h.fileExists("file2.txt"))

	content1 := h.readFile("file1.txt")
	assert.Equal(t, "first\n", content1)

	content2 := h.readFile("file2.txt")
	assert.Equal(t, "second\n", content2)
}

func TestApplyMailbox_Conflict(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	repo := h.initRepo()

	h.commitFile("README.md", "# Test Repository\n\nConflicting content.\n", "Create conflict")

	patch := `From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001
From: Patch Author <author@example.com>
Date: Mon, 1 Jan 2024 12:00:00 +0000
Subject: [PATCH] Modify README

---
 README.md | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)

diff --git a/README.md b/README.md
index 1234567..abcdefg 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,3 @@
 # Test Repository

-Initial content.
+Different content.
--
2.40.0
`

	err := repo.applyMailbox(patch)
	assert.Error(t, err)

	var mergeErr *ErrMerge
	assert.ErrorAs(t, err, &mergeErr)
}

func TestParseGitApplyErrors(t *testing.T) {
	tests := []struct {
		name           string
		errorOutput    string
		expectedCount  int
		expectedReason string
	}{
		{
			name:           "file already exists",
			errorOutput:    `error: path/to/file.txt: already exists in working directory`,
			expectedCount:  1,
			expectedReason: "file already exists",
		},
		{
			name:           "file does not exist",
			errorOutput:    `error: path/to/file.txt: does not exist in working tree`,
			expectedCount:  1,
			expectedReason: "file does not exist",
		},
		{
			name: "patch does not apply",
			errorOutput: `error: patch failed: file.txt:10
error: file.txt: patch does not apply`,
			expectedCount:  1,
			expectedReason: "patch does not apply",
		},
		{
			name: "multiple conflicts",
			errorOutput: `error: patch failed: file1.txt:5
error: file1.txt:5: some error
error: patch failed: file2.txt:10
error: file2.txt:10: another error`,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflicts := parseGitApplyErrors(tt.errorOutput)
			assert.Len(t, conflicts, tt.expectedCount)

			if tt.expectedReason != "" && len(conflicts) > 0 {
				assert.Equal(t, tt.expectedReason, conflicts[0].Reason)
			}
		})
	}
}

func TestErrMerge_Error(t *testing.T) {
	tests := []struct {
		name        string
		err         ErrMerge
		expectedMsg string
	}{
		{
			name: "with conflicts",
			err: ErrMerge{
				Message:     "test merge failed",
				HasConflict: true,
				Conflicts: []ConflictInfo{
					{Filename: "file1.txt", Reason: "conflict 1"},
					{Filename: "file2.txt", Reason: "conflict 2"},
				},
			},
			expectedMsg: "merge failed due to conflicts: test merge failed (2 conflicts)",
		},
		{
			name: "with other error",
			err: ErrMerge{
				Message:    "command failed",
				OtherError: assert.AnError,
			},
			expectedMsg: "merge failed: command failed:",
		},
		{
			name: "message only",
			err: ErrMerge{
				Message: "simple failure",
			},
			expectedMsg: "merge failed: simple failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			assert.Contains(t, errMsg, tt.expectedMsg)
		})
	}
}

func TestMergeWithOptions_Integration(t *testing.T) {
	h := helper(t)
	defer h.cleanup()

	// create a repository first with initial content
	workRepoPath := filepath.Join(h.tempDir, "work-repo")
	workRepo, err := git.PlainInit(workRepoPath, false)
	require.NoError(t, err)

	// configure git user
	cfg, err := workRepo.Config()
	require.NoError(t, err)
	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	err = workRepo.SetConfig(cfg)
	require.NoError(t, err)

	// Create initial commit
	w, err := workRepo.Worktree()
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(workRepoPath, "README.md"), []byte("# Initial\n"), 0644)
	require.NoError(t, err)

	_, err = w.Add("README.md")
	require.NoError(t, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	// create a bare repository (like production)
	bareRepoPath := filepath.Join(h.tempDir, "bare-repo")
	err = InitBare(bareRepoPath, "main")
	require.NoError(t, err)

	// add bare repo as remote and push to it
	_, err = workRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://" + bareRepoPath},
	})
	require.NoError(t, err)

	err = workRepo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{"refs/heads/master:refs/heads/main"},
	})
	require.NoError(t, err)

	// now merge a patch into the bare repo
	gitRepo, err := PlainOpen(bareRepoPath)
	require.NoError(t, err)

	patch := `diff --git a/feature.txt b/feature.txt
new file mode 100644
index 0000000..5e1c309
--- /dev/null
+++ b/feature.txt
@@ -0,0 +1 @@
+Hello World
`

	opts := MergeOptions{
		CommitMessage:  "Add feature",
		CommitterName:  "Test Committer",
		CommitterEmail: "committer@example.com",
		FormatPatch:    false,
	}

	err = gitRepo.MergeWithOptions(patch, "main", opts)
	assert.NoError(t, err)

	// Clone again and verify the changes were merged
	verifyRepoPath := filepath.Join(h.tempDir, "verify-repo")
	verifyRepo, err := git.PlainClone(verifyRepoPath, false, &git.CloneOptions{
		URL: "file://" + bareRepoPath,
	})
	require.NoError(t, err)

	// check that feature.txt exists
	featureFile := filepath.Join(verifyRepoPath, "feature.txt")
	assert.FileExists(t, featureFile)

	content, err := os.ReadFile(featureFile)
	require.NoError(t, err)
	assert.Equal(t, "Hello World\n", string(content))

	// verify commit message
	head, err := verifyRepo.Head()
	require.NoError(t, err)

	commit, err := verifyRepo.CommitObject(head.Hash())
	require.NoError(t, err)
	assert.Equal(t, "Add feature", strings.TrimSpace(commit.Message))
}
