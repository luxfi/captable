# Lux Cap Table

Domain library for cap table management, securities lifecycle, and compliance -- 26 packages, zero database opinion.

```
go get github.com/luxfi/captable
```

## Architecture

`luxfi/captable` models the complete private securities lifecycle as interface-driven Go packages. No ORM, no database driver, no framework. Consumers implement repository interfaces and compose services.

### Packages

| Package | Purpose |
|---------|---------|
| `captable` | Companies, share classes, entries, ownership summaries, fully-diluted calculations |
| `securities` | Security instruments (equity, debt, convertible, warrant), CUSIP/ISIN, issuance/transfer/cancellation/conversion with immutable ledger |
| `stakeholder` | Investors, employees, advisors -- type tracking, accreditation, relationship history |
| `safe` | SAFE agreements (YC post-money templates: cap, discount, MFN, pro rata variants), conversion math |
| `note` | Convertible notes (CCD, OCD, simple), interest accrual (simple/compound, daily through continuous), maturity triggers |
| `warrant` | Common/preferred/broker/penny warrants, cash and cashless exercise |
| `vesting` | Time-based and milestone vesting, cliff, immediate vesting, custom tranches, performance conditions |
| `compliance` | Form D (506b/506c/504), blue sky filings per state, Reg D/Reg S/Rule 144 compliance checks |
| `kyc` | KYC/KYB submissions, AML screening, accreditation verification, pluggable provider interface |
| `omnisub` | Omnibus/sub-account structure (Alpaca OmniSub), positions, orders, corporate actions, tax lots, 1099/K-1 generation |
| `transfer` | Transfer restrictions -- Rule 144, lockup, ROFR, board approval |
| `corporate` | Corporate actions -- stock splits, reverse splits, mergers, spinoffs, reclassification |
| `dividend` | Dividend declaration, record-date processing, distribution calculation |
| `document` | Data rooms, document management, access control, audit trails |
| `disclosure` | PPM/subscription delivery and acknowledgment tracking |
| `comms` | Shareholder notices -- proxy, dividend, regulatory, K-1 |
| `exercise` | Option and warrant exercise processing |
| `conversion` | SAFE and convertible note conversion to equity |
| `settlement` | Trade settlement tracking and reconciliation |
| `scenario` | Waterfall modeling, what-if analysis |
| `valuation` | 409A valuation support, FMV tracking |
| `tax` | 1099-DIV, 1099-B, Schedule K-1 generation |
| `voting` | Shareholder proposals, quorum, vote tallying |
| `waterfall` | Liquidation waterfall -- seniority, participation, caps |
| `edgar` | SEC EDGAR Form D adapter — XML schema marshal, SGML envelope, CIK/CCC auth, EDGARLink Online submission, amendment, status query |
| `bluesky` | State blue-sky notice adapter — NASAA EFD (49 states + DC) + real state-portal adapters (FL, TX, NY, CA, MA), per-state fee calculator |

### Securities Lifecycle

The library models the full lifecycle of private securities:

```
Authorization → Issuance → Vesting → Exercise/Conversion → Transfer → Cancellation
                  │                       │                    │
                  ├── SAFE ──────────────►├── Restriction ─────┤
                  ├── Convertible Note ──►│   Check            │
                  ├── Warrant ───────────►│   (Rule 144,       │
                  └── Option Grant ──────►│    Lockup, ROFR)   │
                                          │                    │
                                          └── On-chain ledger ─┘
```

### Compliance Engine

`pkg/compliance` enforces securities exemption requirements:

**Regulation D**:
- 506(b): max 35 non-accredited investors, no general solicitation, pre-existing relationship required
- 506(c): all investors must be verified accredited (third-party verification), general solicitation permitted

**Regulation S**: Offshore sale verification, directed selling restrictions, compliance period tracking (40 or 365 days by category)

**Rule 144**: Holding period (6 months reporting, 12 months non-reporting), volume limits for affiliates, manner of sale requirements, Form 144 filing tracking

**Blue Sky**: Per-state filing tracking with status, fees, and expiration

### OmniSub (Omnibus/Sub-Account)

`pkg/omnisub` implements the Alpaca OmniSub model for broker-dealer integration:

- Omnibus accounts with per-stakeholder sub-accounts
- Position tracking with cost basis
- Order lifecycle (pending, filled, partial, cancelled, rejected)
- Corporate actions applied to positions
- Tax lot tracking (FIFO, specific identification)
- Annual tax statement generation (1099-B, 1099-DIV, K-1)

## Usage

```go
import (
    "github.com/luxfi/captable/pkg/captable"
    "github.com/luxfi/captable/pkg/securities"
    "github.com/luxfi/captable/pkg/safe"
    "github.com/luxfi/captable/pkg/compliance"
)

// Implement repository interfaces with your database.
repo := myPostgresRepo{}

// Create and manage a company.
company := &captable.Company{
    Name:         "Acme Corp",
    EntityType:   "corporation",
    Jurisdiction: "DE",
}

// Issue a SAFE.
s := &safe.Safe{
    Type:         safe.PostMoney,
    Template:     safe.TemplatePostMoneyCap,
    Capital:      500000,
    ValuationCap: ptr(10000000.0),
    ProRata:      true,
}

// Run a Reg D compliance check.
check := &compliance.RegDCheck{
    Exemption:           "506c",
    AllVerifiedAccredited: true,
    TotalInvestors:       42,
    AccreditedInvestors:  42,
}
```

## Testing

```bash
go test ./...
```

## Papers

- [Lux Threshold MPC](https://github.com/luxfi/papers/blob/main/lux-threshold-mpc.pdf) -- on-chain ledger signing
- [Lux FHE MPC Hybrid](https://github.com/luxfi/papers/blob/main/lux-fhe-mpc-hybrid.pdf) -- encrypted computation over securities data

## License

Lux Ecosystem License v1.2.
