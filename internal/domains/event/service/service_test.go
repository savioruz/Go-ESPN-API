package service

import (
	"context"
	"testing"

	"go-espn-api/infras/otel/mocks"
	eventModel "go-espn-api/internal/domains/event/model"
	"go-espn-api/internal/domains/event/model/dto"
	"go-espn-api/internal/domains/event/repository"

	"github.com/jmoiron/sqlx"
)

// fakeRepo records how many times the competitors query runs so the test can
// assert the list path fetches all competitors in a single query (no N+1).
type fakeRepo struct {
	events        []dto.EventListRow
	competitors   []dto.CompetitorRow
	competitorHit int
	lastIDs       []int64
}

func (f *fakeRepo) List(_ context.Context, _ repository.ListFilter, _, _ int) ([]dto.EventListRow, error) {
	return f.events, nil
}

func (f *fakeRepo) Count(_ context.Context, _ repository.ListFilter) (int, error) {
	return len(f.events), nil
}

func (f *fakeRepo) GetByID(_ context.Context, _ int64) (*dto.EventDetailRow, error) {
	return nil, nil
}

func (f *fakeRepo) GetByESPNID(_ context.Context, _ string) (*dto.EventDetailRow, error) {
	return nil, nil
}

func (f *fakeRepo) CompetitorsByEventIDs(_ context.Context, ids []int64) ([]dto.CompetitorRow, error) {
	f.competitorHit++
	f.lastIDs = ids

	return f.competitors, nil
}

func (f *fakeRepo) StuckEvents(_ context.Context, _, _ int) ([]repository.StuckEventRef, error) {
	return nil, nil
}

func (f *fakeRepo) Upsert(_ context.Context, _ eventModel.Event) (repository.UpsertResult, error) {
	return repository.UpsertResult{}, nil
}

func (f *fakeRepo) UpsertTx(_ context.Context, _ *sqlx.Tx, _ eventModel.Event) (repository.UpsertResult, error) {
	return repository.UpsertResult{}, nil
}

func TestListNoNPlusOne(t *testing.T) {
	repo := &fakeRepo{
		events: []dto.EventListRow{
			{ID: 1, ESPNID: "e1"},
			{ID: 2, ESPNID: "e2"},
			{ID: 3, ESPNID: "e3"},
		},
		competitors: []dto.CompetitorRow{
			{EventID: 1, ID: 10, Order: 0},
			{EventID: 1, ID: 11, Order: 1},
			{EventID: 3, ID: 30, Order: 0},
		},
	}

	svc := New(repo, mocks.NewOtel())

	res, total, err := svc.List(context.Background(), repository.ListFilter{}, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}

	// Exactly one competitors query for the whole page.
	if repo.competitorHit != 1 {
		t.Fatalf("expected 1 competitors query (no N+1), got %d", repo.competitorHit)
	}

	// It was asked for every event id on the page in one call.
	if len(repo.lastIDs) != 3 {
		t.Fatalf("expected all 3 event ids in one query, got %v", repo.lastIDs)
	}

	// Grouping wired competitors to the right events and non-nil elsewhere.
	if len(res[0].Competitors) != 2 {
		t.Fatalf("event 1 should have 2 competitors, got %d", len(res[0].Competitors))
	}

	if res[1].Competitors == nil || len(res[1].Competitors) != 0 {
		t.Fatalf("event 2 should serialise empty competitors slice, got %v", res[1].Competitors)
	}

	if len(res[2].Competitors) != 1 {
		t.Fatalf("event 3 should have 1 competitor, got %d", len(res[2].Competitors))
	}
}
