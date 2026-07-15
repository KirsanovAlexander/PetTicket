package tickets

import (
	"context"
	"errors"
	"testing"

	domain "pet-ticket/internal/domain/tickets"
)

type mockCommentsRepository struct {
	createFunc        func(ctx context.Context, input domain.AddCommentInput) (domain.TicketComment, error)
	getByTicketIDFunc func(ctx context.Context, filter domain.ListCommentsFilter) ([]domain.TicketComment, error)
	getLastByTicketID func(ctx context.Context, ticketID int64) (*domain.TicketComment, error)
	updateFunc        func(ctx context.Context, input domain.UpdateCommentInput) error
	deleteFunc        func(ctx context.Context, id int64) error
	created           []domain.AddCommentInput
}

func (m *mockCommentsRepository) Create(ctx context.Context, input domain.AddCommentInput) (domain.TicketComment, error) {
	m.created = append(m.created, input)
	if m.createFunc != nil {
		return m.createFunc(ctx, input)
	}
	return domain.TicketComment{TicketID: input.TicketID, UserID: input.UserID, Content: input.Content, IsInternal: input.IsInternal}, nil
}

func (m *mockCommentsRepository) GetByTicketID(ctx context.Context, filter domain.ListCommentsFilter) ([]domain.TicketComment, error) {
	if m.getByTicketIDFunc != nil {
		return m.getByTicketIDFunc(ctx, filter)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCommentsRepository) GetLastByTicketID(ctx context.Context, ticketID int64) (*domain.TicketComment, error) {
	if m.getLastByTicketID != nil {
		return m.getLastByTicketID(ctx, ticketID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCommentsRepository) Update(ctx context.Context, input domain.UpdateCommentInput) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, input)
	}
	return errors.New("not implemented")
}

func (m *mockCommentsRepository) Delete(ctx context.Context, id int64) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return errors.New("not implemented")
}

func ticketForComments() domain.Ticket {
	return domain.Ticket{
		ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "old comment",
	}
}

func TestAddComment_EmptyContent_ReturnsError(t *testing.T) {
	svc := NewService(&mockRepository{}, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	_, err := svc.AddComment(context.Background(), domain.AddCommentInput{TicketID: 1, UserID: 100, Content: ""})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestAddComment_DualWrite_CreatesInNewStoreAndLegacyField(t *testing.T) {
	existing := ticketForComments()
	var updatedTicket domain.Ticket

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			updatedTicket = ticket
			return ticket, nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error { return nil },
	}
	comments := &mockCommentsRepository{}
	svc := NewService(repo, comments, &mockDB{}, testLogger(), nil, nil, false)

	_, err := svc.AddComment(context.Background(), domain.AddCommentInput{
		TicketID: 1, UserID: 100, Content: "new comment", IsInternal: false,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(comments.created) != 1 {
		t.Fatalf("expected 1 comment created in new store, got %d", len(comments.created))
	}
	if comments.created[0].Content != "new comment" {
		t.Errorf("expected content 'new comment', got %q", comments.created[0].Content)
	}
	if updatedTicket.Comment != "new comment" {
		t.Errorf("expected legacy tickets.comment updated to 'new comment', got %q", updatedTicket.Comment)
	}
}

func TestAddComment_PublicComment_SetsFirstResponse(t *testing.T) {
	existing := ticketForComments()
	repo := &mockRepository{
		getByIDFunc:    func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
		updateFunc:     func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
		addHistoryFunc: func(ctx context.Context, history domain.History) error { return nil },
	}
	svc := NewService(repo, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	updated, err := svc.AddComment(context.Background(), domain.AddCommentInput{
		TicketID: 1, UserID: 100, Content: "public reply", IsInternal: false,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated.FirstResponseAt == nil {
		t.Error("expected FirstResponseAt to be set for a public (non-internal) comment")
	}
}

func TestAddComment_InternalNote_DoesNotSetFirstResponse(t *testing.T) {
	existing := ticketForComments()
	repo := &mockRepository{
		getByIDFunc:    func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
		updateFunc:     func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
		addHistoryFunc: func(ctx context.Context, history domain.History) error { return nil },
	}
	svc := NewService(repo, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	updated, err := svc.AddComment(context.Background(), domain.AddCommentInput{
		TicketID: 1, UserID: 100, Content: "internal note", IsInternal: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated.FirstResponseAt != nil {
		t.Error("expected FirstResponseAt to stay unset for an internal note")
	}
}

func TestAddComment_InternalNote_UpdatesUserActivity(t *testing.T) {
	existing := ticketForComments()
	activityUpdated := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
		updateLastUserActivityFunc: func(ctx context.Context, ticketID int64) error {
			activityUpdated = true
			return nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error { return nil },
	}
	svc := NewService(repo, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	// IsInternal=true => isSupportComment=false => расценивается как активность клиента.
	if _, err := svc.AddComment(context.Background(), domain.AddCommentInput{
		TicketID: 1, UserID: 100, Content: "internal note", IsInternal: true,
	}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !activityUpdated {
		t.Error("expected UpdateLastUserActivity to be called")
	}
}

func TestAddComment_PublicComment_DoesNotUpdateUserActivity(t *testing.T) {
	existing := ticketForComments()
	activityUpdated := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
		updateLastUserActivityFunc: func(ctx context.Context, ticketID int64) error {
			activityUpdated = true
			return nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error { return nil },
	}
	svc := NewService(repo, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	if _, err := svc.AddComment(context.Background(), domain.AddCommentInput{
		TicketID: 1, UserID: 100, Content: "public reply", IsInternal: false,
	}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if activityUpdated {
		t.Error("expected UpdateLastUserActivity NOT to be called for a public (support) comment")
	}
}

func TestGetComments_UseNewComments_True_DelegatesToRepo(t *testing.T) {
	expected := []domain.TicketComment{{ID: 1, TicketID: 1, Content: "a"}}
	var capturedFilter domain.ListCommentsFilter
	comments := &mockCommentsRepository{
		getByTicketIDFunc: func(ctx context.Context, filter domain.ListCommentsFilter) ([]domain.TicketComment, error) {
			capturedFilter = filter
			return expected, nil
		},
	}
	svc := NewService(&mockRepository{}, comments, &mockDB{}, testLogger(), nil, nil, true)

	result, err := svc.GetComments(context.Background(), domain.ListCommentsFilter{TicketID: 1, IncludeInternal: true})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 1 || result[0].Content != "a" {
		t.Errorf("expected result from commentsRepo, got: %+v", result)
	}
	if capturedFilter.TicketID != 1 || !capturedFilter.IncludeInternal {
		t.Errorf("unexpected filter passed through: %+v", capturedFilter)
	}
}

func TestGetComments_UseNewComments_False_FallsBackToLegacyField(t *testing.T) {
	existing := ticketForComments()
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
	}
	svc := NewService(repo, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	result, err := svc.GetComments(context.Background(), domain.ListCommentsFilter{TicketID: 1})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 legacy comment wrapped, got %d", len(result))
	}
	if result[0].Content != "old comment" {
		t.Errorf("expected legacy comment content 'old comment', got %q", result[0].Content)
	}
}

func TestGetComments_UseNewComments_False_EmptyLegacyComment_ReturnsNil(t *testing.T) {
	existing := ticketForComments()
	existing.Comment = ""
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
	}
	svc := NewService(repo, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	result, err := svc.GetComments(context.Background(), domain.ListCommentsFilter{TicketID: 1})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result for ticket with no legacy comment, got %d", len(result))
	}
}

func TestGetLastComment_UseNewComments_True_DelegatesToRepo(t *testing.T) {
	expected := &domain.TicketComment{ID: 5, TicketID: 1, Content: "last"}
	comments := &mockCommentsRepository{
		getLastByTicketID: func(ctx context.Context, ticketID int64) (*domain.TicketComment, error) {
			return expected, nil
		},
	}
	svc := NewService(&mockRepository{}, comments, &mockDB{}, testLogger(), nil, nil, true)

	result, err := svc.GetLastComment(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil || result.ID != 5 {
		t.Errorf("expected comment from commentsRepo, got: %+v", result)
	}
}

func TestGetLastComment_UseNewComments_False_FallsBackToLegacyField(t *testing.T) {
	existing := ticketForComments()
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existing, nil },
	}
	svc := NewService(repo, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	result, err := svc.GetLastComment(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil || result.Content != "old comment" {
		t.Errorf("expected legacy fallback comment, got: %+v", result)
	}
}

func TestUpdateComment_EmptyContent_ReturnsError(t *testing.T) {
	svc := NewService(&mockRepository{}, &mockCommentsRepository{}, &mockDB{}, testLogger(), nil, nil, false)

	err := svc.UpdateComment(context.Background(), domain.UpdateCommentInput{ID: 1, Content: ""})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestUpdateComment_DelegatesToRepo(t *testing.T) {
	var captured domain.UpdateCommentInput
	comments := &mockCommentsRepository{
		updateFunc: func(ctx context.Context, input domain.UpdateCommentInput) error {
			captured = input
			return nil
		},
	}
	svc := NewService(&mockRepository{}, comments, &mockDB{}, testLogger(), nil, nil, false)

	if err := svc.UpdateComment(context.Background(), domain.UpdateCommentInput{ID: 7, Content: "edited"}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if captured.ID != 7 || captured.Content != "edited" {
		t.Errorf("unexpected input passed through: %+v", captured)
	}
}

func TestDeleteComment_DelegatesToRepo(t *testing.T) {
	var capturedID int64
	comments := &mockCommentsRepository{
		deleteFunc: func(ctx context.Context, id int64) error {
			capturedID = id
			return nil
		},
	}
	svc := NewService(&mockRepository{}, comments, &mockDB{}, testLogger(), nil, nil, false)

	if err := svc.DeleteComment(context.Background(), 42); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedID != 42 {
		t.Errorf("expected delete called with id 42, got %d", capturedID)
	}
}
