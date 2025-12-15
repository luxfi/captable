package voting

import (
	"context"
	"fmt"
	"time"
)

// Service provides proxy voting operations.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateProposal creates a new shareholder voting proposal.
func (s *Service) CreateProposal(ctx context.Context, p *Proposal) error {
	if p.Title == "" {
		return fmt.Errorf("title required")
	}
	if p.CompanyID == "" {
		return fmt.Errorf("company_id required")
	}
	if len(p.ShareClassIDs) == 0 {
		return fmt.Errorf("at least one share_class_id required")
	}
	if p.QuorumPercent <= 0 || p.QuorumPercent > 100 {
		return fmt.Errorf("quorum_percent must be between 0 and 100")
	}
	if p.Status == "" {
		p.Status = "draft"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	return s.repo.CreateProposal(ctx, p)
}

// GetProposal returns a proposal by ID.
func (s *Service) GetProposal(ctx context.Context, id string) (*Proposal, error) {
	return s.repo.GetProposal(ctx, id)
}

// ListProposals returns proposals matching the filter.
func (s *Service) ListProposals(ctx context.Context, filter ProposalFilter) ([]*Proposal, error) {
	return s.repo.ListProposals(ctx, filter)
}

// CastVote records a stakeholder's vote on a proposal.
func (s *Service) CastVote(ctx context.Context, v *Vote) error {
	if v.ProposalID == "" {
		return fmt.Errorf("proposal_id required")
	}
	if v.StakeholderID == "" {
		return fmt.Errorf("stakeholder_id required")
	}
	if v.Choice != "for" && v.Choice != "against" && v.Choice != "abstain" {
		return fmt.Errorf("choice must be 'for', 'against', or 'abstain'")
	}
	if v.SharesVoted <= 0 {
		return fmt.Errorf("shares_voted must be positive")
	}

	// Verify proposal is open
	p, err := s.repo.GetProposal(ctx, v.ProposalID)
	if err != nil {
		return err
	}
	if p.Status != "open" {
		return fmt.Errorf("proposal %q is %s, not open for voting", v.ProposalID, p.Status)
	}

	if v.CastAt.IsZero() {
		v.CastAt = time.Now().UTC()
	}
	return s.repo.CastVote(ctx, v)
}

// GetResults returns aggregated voting results for a proposal.
func (s *Service) GetResults(ctx context.Context, proposalID string) (*Results, error) {
	return s.repo.GetResults(ctx, proposalID)
}
