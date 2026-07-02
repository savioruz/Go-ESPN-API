package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/espn"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	teamRepo "go-espn-api/internal/domains/team/repository"
	transactionModel "go-espn-api/internal/domains/transaction/model"
	transactionRepo "go-espn-api/internal/domains/transaction/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// TransactionsService ingests league transactions from ESPN.
type TransactionsService struct {
	espn         espn.ESPN
	db           *postgres.Connection
	otel         otel.Otel
	sports       sportRepo.Sport
	leagues      leagueRepo.League
	teams        teamRepo.Team
	transactions transactionRepo.Transaction
}

// NewTransactionsService constructs a TransactionsService.
func NewTransactionsService(
	espnClient espn.ESPN,
	db *postgres.Connection,
	otl otel.Otel,
	sports sportRepo.Sport,
	leagues leagueRepo.League,
	teams teamRepo.Team,
	transactions transactionRepo.Transaction,
) *TransactionsService {
	return &TransactionsService{espn: espnClient, db: db, otel: otl, sports: sports, leagues: leagues, teams: teams, transactions: transactions}
}

// IngestTransactions fetches and upserts recent transactions for a sport/league.
func (s *TransactionsService) IngestTransactions(ctx context.Context, sport, league string) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.Transactions")
	defer scope.End()

	scope.SetAttributes(map[string]any{"sport": sport, "league": league})

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		leagueID, err := resolveSportLeague(ctx, tx, s.sports, s.leagues, sport, league)
		if err != nil {
			return err
		}

		resp, err := s.espn.GetLeagueTransactions(ctx, sport, league)
		if err != nil {
			return fmt.Errorf("fetch transactions: %w", err)
		}

		var root map[string]any
		if err := json.Unmarshal(resp.Data, &root); err != nil {
			return fmt.Errorf("decode transactions: %w", err)
		}

		items := mslice(root, "items")
		if len(items) == 0 {
			items = mslice(root, "transactions")
		}

		if len(items) == 0 {
			log.Info().Str("sport", sport).Str("league", league).Msg("no_transactions_found")

			return nil
		}

		for _, raw := range items {
			m, teamESPNID, ok := parseTransaction(leagueID, asMap(raw))
			if !ok {
				result.Errors++

				continue
			}

			teamID, err := s.teams.IDByESPNInLeagueTx(ctx, tx, leagueID, teamESPNID)
			if err != nil {
				return err
			}

			m.TeamID = teamID

			created := true
			if m.ESPNID != "" {
				created, err = s.transactions.UpsertByESPNTx(ctx, tx, m)
			} else {
				err = s.transactions.InsertTx(ctx, tx, m)
			}

			if err != nil {
				return err
			}

			if created {
				result.Created++
			} else {
				result.Updated++
			}
		}

		return nil
	})
	if err != nil {
		scope.TraceError(err)

		return IngestionResult{}, fmt.Errorf("failed to ingest transactions: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated, "errors": result.Errors})

	return result, nil
}

// parseTransaction mirrors _parse_transaction. ok=false when there is no
// description. The returned string is the team's ESPN id for later resolution.
func parseTransaction(leagueID int64, item map[string]any) (transactionModel.Transaction, string, bool) {
	description := mstrOr(item, "description", "text")
	if description == "" {
		return transactionModel.Transaction{}, "", false
	}

	athlete := mmap(item, "athlete")

	return transactionModel.Transaction{
		LeagueID:      leagueID,
		ESPNID:        mid(item, "id"),
		Date:          parseDateOnly(mstr(item, "date")),
		Description:   description,
		Type:          mstr(item, "type"),
		AthleteName:   mstr(athlete, "displayName"),
		AthleteESPNID: mid(athlete, "id"),
		RawData:       rawObj(item, "{}"),
	}, mid(mmap(item, "team"), "id"), true
}
