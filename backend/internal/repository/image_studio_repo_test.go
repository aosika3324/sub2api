//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

type ImageStudioRepoSuite struct {
	suite.Suite
	ctx    context.Context
	client *dbent.Client
	repo   service.ImageStudioRepository
}

func (s *ImageStudioRepoSuite) SetupTest() {
	s.ctx = context.Background()
	tx := testEntTx(s.T())
	s.client = tx.Client()
	s.repo = NewImageStudioRepository(s.client)
}

func TestImageStudioRepoSuite(t *testing.T) {
	suite.Run(t, new(ImageStudioRepoSuite))
}

// --- helpers ---

func (s *ImageStudioRepoSuite) mustCreateUser(email string) *dbent.User {
	s.T().Helper()
	u, err := s.client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(s.ctx)
	s.Require().NoError(err, "create user")
	return u
}

// --- conversation tests ---

func (s *ImageStudioRepoSuite) TestCreateConversation() {
	u := s.mustCreateUser("conv-create@test.com")

	conv, err := s.repo.CreateConversation(s.ctx, u.ID, "My Conversation")
	s.Require().NoError(err)
	s.Require().NotZero(conv.ID)
	s.Require().Equal(u.ID, conv.UserID)
	s.Require().Equal("My Conversation", conv.Title)
	s.Require().False(conv.CreatedAt.IsZero())
}

func (s *ImageStudioRepoSuite) TestListConversations_Pagination() {
	u := s.mustCreateUser("conv-list@test.com")

	for i := 0; i < 5; i++ {
		_, err := s.repo.CreateConversation(s.ctx, u.ID, "conv")
		s.Require().NoError(err)
	}

	items, total, err := s.repo.ListConversations(s.ctx, u.ID, 1, 2)
	s.Require().NoError(err)
	s.Require().Equal(5, total)
	s.Require().Len(items, 2)

	items2, total2, err := s.repo.ListConversations(s.ctx, u.ID, 3, 2)
	s.Require().NoError(err)
	s.Require().Equal(5, total2)
	s.Require().Len(items2, 1)
}

func (s *ImageStudioRepoSuite) TestListConversations_UserScoping() {
	u1 := s.mustCreateUser("conv-scope-u1@test.com")
	u2 := s.mustCreateUser("conv-scope-u2@test.com")

	_, err := s.repo.CreateConversation(s.ctx, u1.ID, "u1-conv")
	s.Require().NoError(err)
	_, err = s.repo.CreateConversation(s.ctx, u2.ID, "u2-conv")
	s.Require().NoError(err)

	items, total, err := s.repo.ListConversations(s.ctx, u1.ID, 1, 10)
	s.Require().NoError(err)
	s.Require().Equal(1, total)
	s.Require().Len(items, 1)
	s.Require().Equal(u1.ID, items[0].UserID)
}

func (s *ImageStudioRepoSuite) TestListConversations_ExcludesSoftDeleted() {
	u := s.mustCreateUser("conv-softdel@test.com")

	kept, err := s.repo.CreateConversation(s.ctx, u.ID, "kept")
	s.Require().NoError(err)
	deleted, err := s.repo.CreateConversation(s.ctx, u.ID, "deleted")
	s.Require().NoError(err)

	// Soft-delete one row directly via the ent client. The soft-delete hook
	// converts DeleteOneID into an UPDATE that sets deleted_at.
	s.Require().NoError(s.client.ImageConversation.DeleteOneID(deleted.ID).Exec(s.ctx))

	items, total, err := s.repo.ListConversations(s.ctx, u.ID, 1, 10)
	s.Require().NoError(err)
	s.Require().Equal(1, total, "soft-deleted conversation should not be counted")
	s.Require().Len(items, 1)
	s.Require().Equal(kept.ID, items[0].ID)
}

// --- generation tests ---

func (s *ImageStudioRepoSuite) newTestGeneration(userID, convID, groupID int64) *dbent.ImageGeneration {
	return &dbent.ImageGeneration{
		UserID:         userID,
		ConversationID: convID,
		GroupID:        groupID,
		Prompt:         "a cat on a mars",
		Model:          "dall-e-3",
		Size:           "1024x1024",
		Quality:        "standard",
		N:              1,
		ImageCount:     1,
		Status:         "pending",
	}
}

func (s *ImageStudioRepoSuite) mustCreateGroup(name string) *dbent.Group {
	s.T().Helper()
	g, err := s.client.Group.Create().
		SetName(name).
		SetStatus(service.StatusActive).
		Save(s.ctx)
	s.Require().NoError(err, "create group")
	return g
}

func (s *ImageStudioRepoSuite) TestCreateGeneration() {
	u := s.mustCreateUser("gen-create@test.com")
	g := s.mustCreateGroup("gen-group-create")
	conv, err := s.repo.CreateConversation(s.ctx, u.ID, "conv")
	s.Require().NoError(err)

	gen, err := s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u.ID, conv.ID, g.ID))
	s.Require().NoError(err)
	s.Require().NotZero(gen.ID)
	s.Require().Equal(u.ID, gen.UserID)
	s.Require().Equal(conv.ID, gen.ConversationID)
	s.Require().Equal("pending", gen.Status)
	s.Require().Equal("a cat on a mars", gen.Prompt)
}

func (s *ImageStudioRepoSuite) TestUpdateGenerationStatus() {
	u := s.mustCreateUser("gen-update@test.com")
	g := s.mustCreateGroup("gen-group-update")
	conv, err := s.repo.CreateConversation(s.ctx, u.ID, "conv")
	s.Require().NoError(err)

	gen, err := s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u.ID, conv.ID, g.ID))
	s.Require().NoError(err)

	err = s.repo.UpdateGenerationStatus(
		s.ctx,
		gen.ID,
		"done",
		[]string{"s3://bucket/img1.png"},
		0.04,
		1,
		1024,
		1024,
		"",
	)
	s.Require().NoError(err)

	got, err := s.repo.GetGeneration(s.ctx, gen.ID)
	s.Require().NoError(err)
	s.Require().Equal("done", got.Status)
	s.Require().InDelta(0.04, got.Cost, 1e-9)
	s.Require().Equal([]string{"s3://bucket/img1.png"}, got.StorageKeys)
	s.Require().Equal(1, got.ImageCount)
	s.Require().NotNil(got.Width)
	s.Require().Equal(1024, *got.Width)
	s.Require().NotNil(got.Height)
	s.Require().Equal(1024, *got.Height)
	s.Require().Nil(got.Error)
}

func (s *ImageStudioRepoSuite) TestUpdateGenerationStatus_Error() {
	u := s.mustCreateUser("gen-err@test.com")
	g := s.mustCreateGroup("gen-group-err")
	conv, err := s.repo.CreateConversation(s.ctx, u.ID, "conv")
	s.Require().NoError(err)

	gen, err := s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u.ID, conv.ID, g.ID))
	s.Require().NoError(err)

	err = s.repo.UpdateGenerationStatus(s.ctx, gen.ID, "failed", nil, 0, 0, 0, 0, "upstream error")
	s.Require().NoError(err)

	got, err := s.repo.GetGeneration(s.ctx, gen.ID)
	s.Require().NoError(err)
	s.Require().Equal("failed", got.Status)
	s.Require().NotNil(got.Error)
	s.Require().Equal("upstream error", *got.Error)
}

func (s *ImageStudioRepoSuite) TestGetGeneration_NotFound() {
	_, err := s.repo.GetGeneration(s.ctx, 999999999)
	s.Require().Error(err)
}

func (s *ImageStudioRepoSuite) TestListGenerations_ByUser() {
	u1 := s.mustCreateUser("genlist-u1@test.com")
	u2 := s.mustCreateUser("genlist-u2@test.com")
	g := s.mustCreateGroup("gen-group-list")
	conv1, err := s.repo.CreateConversation(s.ctx, u1.ID, "c1")
	s.Require().NoError(err)
	conv2, err := s.repo.CreateConversation(s.ctx, u2.ID, "c2")
	s.Require().NoError(err)

	for i := 0; i < 3; i++ {
		_, err := s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u1.ID, conv1.ID, g.ID))
		s.Require().NoError(err)
	}
	_, err = s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u2.ID, conv2.ID, g.ID))
	s.Require().NoError(err)

	items, total, err := s.repo.ListGenerations(s.ctx, u1.ID, nil, 1, 10)
	s.Require().NoError(err)
	s.Require().Equal(3, total)
	s.Require().Len(items, 3)
	for _, it := range items {
		s.Require().Equal(u1.ID, it.UserID)
	}
}

func (s *ImageStudioRepoSuite) TestListGenerations_ByConversation() {
	u := s.mustCreateUser("genlist-conv@test.com")
	g := s.mustCreateGroup("gen-group-conv")
	conv1, err := s.repo.CreateConversation(s.ctx, u.ID, "c1")
	s.Require().NoError(err)
	conv2, err := s.repo.CreateConversation(s.ctx, u.ID, "c2")
	s.Require().NoError(err)

	for i := 0; i < 2; i++ {
		_, err := s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u.ID, conv1.ID, g.ID))
		s.Require().NoError(err)
	}
	_, err = s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u.ID, conv2.ID, g.ID))
	s.Require().NoError(err)

	items, total, err := s.repo.ListGenerations(s.ctx, u.ID, &conv1.ID, 1, 10)
	s.Require().NoError(err)
	s.Require().Equal(2, total)
	s.Require().Len(items, 2)
	for _, it := range items {
		s.Require().Equal(conv1.ID, it.ConversationID)
	}
}

func (s *ImageStudioRepoSuite) TestListGenerations_Pagination() {
	u := s.mustCreateUser("genlist-page@test.com")
	g := s.mustCreateGroup("gen-group-page")
	conv, err := s.repo.CreateConversation(s.ctx, u.ID, "c")
	s.Require().NoError(err)

	for i := 0; i < 5; i++ {
		_, err := s.repo.CreateGeneration(s.ctx, s.newTestGeneration(u.ID, conv.ID, g.ID))
		s.Require().NoError(err)
	}

	items, total, err := s.repo.ListGenerations(s.ctx, u.ID, nil, 1, 2)
	s.Require().NoError(err)
	s.Require().Equal(5, total)
	s.Require().Len(items, 2)
}

func (s *ImageStudioRepoSuite) TestClearUserHistory_SoftDeletesOwnedRowsAndReturnsKeys() {
	u1 := s.mustCreateUser("clear-history-u1@test.com")
	u2 := s.mustCreateUser("clear-history-u2@test.com")
	g := s.mustCreateGroup("clear-history-group")
	conv1, err := s.repo.CreateConversation(s.ctx, u1.ID, "u1")
	s.Require().NoError(err)
	conv2, err := s.repo.CreateConversation(s.ctx, u2.ID, "u2")
	s.Require().NoError(err)

	gen1 := s.newTestGeneration(u1.ID, conv1.ID, g.ID)
	gen1.StorageKeys = []string{"u1/out.png"}
	gen1.InputStorageKeys = []string{"u1/input.png"}
	_, err = s.repo.CreateGeneration(s.ctx, gen1)
	s.Require().NoError(err)

	gen2 := s.newTestGeneration(u2.ID, conv2.ID, g.ID)
	gen2.StorageKeys = []string{"u2/out.png"}
	_, err = s.repo.CreateGeneration(s.ctx, gen2)
	s.Require().NoError(err)

	keys, err := s.repo.ClearUserHistory(s.ctx, u1.ID)
	s.Require().NoError(err)
	s.Require().ElementsMatch([]string{"u1/out.png", "u1/input.png"}, keys)

	convs, total, err := s.repo.ListConversations(s.ctx, u1.ID, 1, 10)
	s.Require().NoError(err)
	s.Require().Equal(0, total)
	s.Require().Empty(convs)
	gens, total, err := s.repo.ListGenerations(s.ctx, u1.ID, nil, 1, 10)
	s.Require().NoError(err)
	s.Require().Equal(0, total)
	s.Require().Empty(gens)

	gens, total, err = s.repo.ListGenerations(s.ctx, u2.ID, nil, 1, 10)
	s.Require().NoError(err)
	s.Require().Equal(1, total)
	s.Require().Len(gens, 1)
}
