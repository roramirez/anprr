package tui

import (
	"errors"
	"testing"

	"github.com/roramirez/anprr/internal/github"
)

func submittingApp() AppModel {
	d := loadedDetail()
	d.state = detailStateSubmitting
	return AppModel{
		active: screenDetail,
		cache:  github.NewCache(),
		detail: d,
	}
}

// handleReviewDone

func TestApp_handleReviewDone_success_resetsToReady(t *testing.T) {
	m := submittingApp()
	got, _ := m.handleReviewDone(ReviewDoneMsg{})
	if got.(AppModel).detail.state != detailStateReady {
		t.Errorf("expected detailStateReady after successful review, got %d", got.(AppModel).detail.state)
	}
}

func TestApp_handleReviewDone_error_resetsToReady(t *testing.T) {
	m := submittingApp()
	got, _ := m.handleReviewDone(ReviewDoneMsg{Err: errors.New("forbidden")})
	if got.(AppModel).detail.state != detailStateReady {
		t.Errorf("expected detailStateReady after failed review, got %d", got.(AppModel).detail.state)
	}
}

// handleCommentDone

func TestApp_handleCommentDone_success_resetsToReady(t *testing.T) {
	m := submittingApp()
	got, _ := m.handleCommentDone(CommentDoneMsg{})
	if got.(AppModel).detail.state != detailStateReady {
		t.Errorf("expected detailStateReady after successful comment, got %d", got.(AppModel).detail.state)
	}
}

func TestApp_handleCommentDone_error_resetsToReady(t *testing.T) {
	m := submittingApp()
	got, _ := m.handleCommentDone(CommentDoneMsg{Err: errors.New("network error")})
	if got.(AppModel).detail.state != detailStateReady {
		t.Errorf("expected detailStateReady after failed comment, got %d", got.(AppModel).detail.state)
	}
}
