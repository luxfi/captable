package voting

import "time"

// Proposal is a shareholder voting proposal (board election, merger, amendment, etc.).
type Proposal struct {
	ID            string    `json:"id"`
	CompanyID     string    `json:"company_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Type          string    `json:"type"` // board_election, amendment, merger, general
	ShareClassIDs []string  `json:"share_class_ids"`
	RecordDate    time.Time `json:"record_date"`
	Deadline      time.Time `json:"deadline"`
	QuorumPercent float64   `json:"quorum_percent"`
	Status        string    `json:"status"` // draft, open, closed, certified
	CreatedAt     time.Time `json:"created_at"`
}

// Vote is a single shareholder's vote on a proposal.
type Vote struct {
	ID            string    `json:"id"`
	ProposalID    string    `json:"proposal_id"`
	StakeholderID string    `json:"stakeholder_id"`
	Choice        string    `json:"choice"` // for, against, abstain
	SharesVoted   int64     `json:"shares_voted"`
	CastAt        time.Time `json:"cast_at"`
}

// Results is the aggregated voting outcome for a proposal.
type Results struct {
	ProposalID          string `json:"proposal_id"`
	TotalEligibleShares int64  `json:"total_eligible_shares"`
	TotalVotedShares    int64  `json:"total_voted_shares"`
	QuorumMet           bool   `json:"quorum_met"`
	For                 int64  `json:"for"`
	Against             int64  `json:"against"`
	Abstain             int64  `json:"abstain"`
	Status              string `json:"status"`
}

// ProposalFilter for listing proposals.
type ProposalFilter struct {
	CompanyID string `json:"company_id,omitempty"`
	Status    string `json:"status,omitempty"`
}
