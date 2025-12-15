package voting

import "context"

// Repository is the storage interface for voting.
type Repository interface {
	CreateProposal(ctx context.Context, p *Proposal) error
	GetProposal(ctx context.Context, id string) (*Proposal, error)
	ListProposals(ctx context.Context, filter ProposalFilter) ([]*Proposal, error)
	UpdateProposal(ctx context.Context, p *Proposal) error

	CastVote(ctx context.Context, v *Vote) error
	ListVotes(ctx context.Context, proposalID string) ([]*Vote, error)
	GetResults(ctx context.Context, proposalID string) (*Results, error)
}
