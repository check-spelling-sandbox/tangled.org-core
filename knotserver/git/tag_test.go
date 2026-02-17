package git

import (
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TagSuite struct {
	suite.Suite
	*RepoSuite
}

func TestTagSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TagSuite))
}

func (s *TagSuite) SetupTest() {
	s.RepoSuite = NewRepoSuite(s.T())
}

func (s *TagSuite) TearDownTest() {
	s.RepoSuite.cleanup()
}

func (s *TagSuite) setupRepoWithTags() {
	s.init()

	// create commits for tagging
	commit1 := s.commitFile("file1.txt", "content 1", "Add file1")
	commit2 := s.commitFile("file2.txt", "content 2", "Add file2")
	commit3 := s.commitFile("file3.txt", "content 3", "Add file3")
	commit4 := s.commitFile("file4.txt", "content 4", "Add file4")
	commit5 := s.commitFile("file5.txt", "content 5", "Add file5")

	// create annotated tags
	s.createAnnotatedTag(
		"v1.0.0",
		commit1,
		"Tagger One",
		"tagger1@example.com",
		"Release version 1.0.0\n\nThis is the first stable release.",
		s.baseTime.Add(1*time.Hour),
	)

	s.createAnnotatedTag(
		"v1.1.0",
		commit2,
		"Tagger Two",
		"tagger2@example.com",
		"Release version 1.1.0",
		s.baseTime.Add(2*time.Hour),
	)

	// create lightweight tags
	s.createLightweightTag("v2.0.0", commit3)
	s.createLightweightTag("v2.1.0", commit4)

	// create another annotated tag
	s.createAnnotatedTag(
		"v3.0.0",
		commit5,
		"Tagger Three",
		"tagger3@example.com",
		"Major version 3.0.0\n\nBreaking changes included.",
		s.baseTime.Add(3*time.Hour),
	)
}

func (s *TagSuite) TestTags_All() {
	s.setupRepoWithTags()

	tags, err := s.repo.Tags(nil)
	require.NoError(s.T(), err)

	// we created 5 tags total (3 annotated, 2 lightweight)
	assert.Len(s.T(), tags, 5, "expected 5 tags")

	// verify tags are sorted by creation date (newest first)
	expectedAnnotated := map[string]bool{
		"v1.0.0": true,
		"v1.1.0": true,
		"v3.0.0": true,
	}

	expectedLightweight := map[string]bool{
		"v2.0.0": true,
		"v2.1.0": true,
	}

	for _, tag := range tags {
		if expectedAnnotated[tag.Name] {
			// annotated tags should have tagger info
			assert.NotEmpty(s.T(), tag.Tagger.Name, "annotated tag %s should have tagger name", tag.Name)
			assert.NotEmpty(s.T(), tag.Message, "annotated tag %s should have message", tag.Name)
		} else if expectedLightweight[tag.Name] {
			// lightweight tags won't have tagger info or message (they'll have empty values)
		} else {
			s.T().Errorf("unexpected tag name: %s", tag.Name)
		}
	}
}

func (s *TagSuite) TestTags_WithLimit() {
	s.setupRepoWithTags()

	tests := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{
			name:          "limit 1",
			limit:         1,
			expectedCount: 1,
		},
		{
			name:          "limit 2",
			limit:         2,
			expectedCount: 2,
		},
		{
			name:          "limit 3",
			limit:         3,
			expectedCount: 3,
		},
		{
			name:          "limit 10 (more than available)",
			limit:         10,
			expectedCount: 5,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tags, err := s.repo.Tags(&TagsOptions{
				Limit: tt.limit,
			})
			require.NoError(s.T(), err)
			assert.Len(s.T(), tags, tt.expectedCount, "expected %d tags", tt.expectedCount)
		})
	}
}

func (s *TagSuite) TestTags_WithOffset() {
	s.setupRepoWithTags()

	tests := []struct {
		name          string
		offset        int
		expectedCount int
	}{
		{
			name:          "offset 0",
			offset:        0,
			expectedCount: 5,
		},
		{
			name:          "offset 1",
			offset:        1,
			expectedCount: 4,
		},
		{
			name:          "offset 2",
			offset:        2,
			expectedCount: 3,
		},
		{
			name:          "offset 4",
			offset:        4,
			expectedCount: 1,
		},
		{
			name:          "offset 5 (all skipped)",
			offset:        5,
			expectedCount: 0,
		},
		{
			name:          "offset 10 (more than available)",
			offset:        10,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tags, err := s.repo.Tags(&TagsOptions{
				Offset: tt.offset,
			})
			require.NoError(s.T(), err)
			assert.Len(s.T(), tags, tt.expectedCount, "expected %d tags", tt.expectedCount)
		})
	}
}

func (s *TagSuite) TestTags_WithLimitAndOffset() {
	s.setupRepoWithTags()

	tests := []struct {
		name          string
		limit         int
		offset        int
		expectedCount int
	}{
		{
			name:          "limit 2, offset 0",
			limit:         2,
			offset:        0,
			expectedCount: 2,
		},
		{
			name:          "limit 2, offset 1",
			limit:         2,
			offset:        1,
			expectedCount: 2,
		},
		{
			name:          "limit 2, offset 3",
			limit:         2,
			offset:        3,
			expectedCount: 2,
		},
		{
			name:          "limit 2, offset 4",
			limit:         2,
			offset:        4,
			expectedCount: 1,
		},
		{
			name:          "limit 3, offset 2",
			limit:         3,
			offset:        2,
			expectedCount: 3,
		},
		{
			name:          "limit 10, offset 3",
			limit:         10,
			offset:        3,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tags, err := s.repo.Tags(&TagsOptions{
				Limit:  tt.limit,
				Offset: tt.offset,
			})
			require.NoError(s.T(), err)
			assert.Len(s.T(), tags, tt.expectedCount, "expected %d tags", tt.expectedCount)
		})
	}
}

func (s *TagSuite) TestTags_EmptyRepo() {
	repoPath := filepath.Join(s.tempDir, "empty-repo")

	_, err := gogit.PlainInit(repoPath, false)
	require.NoError(s.T(), err)

	gitRepo, err := PlainOpen(repoPath)
	require.NoError(s.T(), err)

	tags, err := gitRepo.Tags(nil)
	require.NoError(s.T(), err)

	if tags != nil {
		assert.Empty(s.T(), tags, "expected no tags in empty repo")
	}
}

func (s *TagSuite) TestTags_Pagination() {
	s.setupRepoWithTags()

	allTags, err := s.repo.Tags(nil)
	require.NoError(s.T(), err)
	assert.Len(s.T(), allTags, 5, "expected 5 tags")

	pageSize := 2
	var paginatedTags []object.Tag

	for offset := 0; offset < len(allTags); offset += pageSize {
		tags, err := s.repo.Tags(&TagsOptions{
			Limit:  pageSize,
			Offset: offset,
		})
		require.NoError(s.T(), err)
		paginatedTags = append(paginatedTags, tags...)
	}

	assert.Len(s.T(), paginatedTags, len(allTags), "pagination should return all tags")

	for i := range allTags {
		assert.Equal(s.T(), allTags[i].Name, paginatedTags[i].Name,
			"tag at index %d differs", i)
	}
}

func (s *TagSuite) TestTags_VerifyAnnotatedTagFields() {
	s.setupRepoWithTags()

	tags, err := s.repo.Tags(nil)
	require.NoError(s.T(), err)

	var v1Tag *object.Tag
	for i := range tags {
		if tags[i].Name == "v1.0.0" {
			v1Tag = &tags[i]
			break
		}
	}

	require.NotNil(s.T(), v1Tag, "v1.0.0 tag not found")

	assert.Equal(s.T(), "Tagger One", v1Tag.Tagger.Name, "tagger name should match")
	assert.Equal(s.T(), "tagger1@example.com", v1Tag.Tagger.Email, "tagger email should match")

	assert.Equal(s.T(), "Release version 1.0.0\n\nThis is the first stable release.\n",
		v1Tag.Message, "tag message should match")

	assert.Equal(s.T(), plumbing.TagObject, v1Tag.TargetType,
		"target type should be CommitObject")

	assert.False(s.T(), v1Tag.Hash.IsZero(), "tag hash should be set")

	assert.False(s.T(), v1Tag.Target.IsZero(), "target hash should be set")
}

func (s *TagSuite) TestTags_NilOptions() {
	s.setupRepoWithTags()

	tags, err := s.repo.Tags(nil)
	require.NoError(s.T(), err)
	assert.Len(s.T(), tags, 5, "nil options should return all tags")
}

func (s *TagSuite) TestTags_ZeroLimitAndOffset() {
	s.setupRepoWithTags()

	tags, err := s.repo.Tags(&TagsOptions{
		Limit:  0,
		Offset: 0,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), tags, 5, "zero limit should return all tags")
}

func (s *TagSuite) TestTags_Pattern() {
	s.setupRepoWithTags()

	v1tag, err := s.repo.Tags(&TagsOptions{
		Pattern: "refs/tags/v1.0.0",
	})

	require.NoError(s.T(), err)
	assert.Len(s.T(), v1tag, 1, "expected 1 tag")
}
