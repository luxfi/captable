# Lux Cap Table

Core Transfer Agent (TA) engine as a reusable Go package. No database layer -- consumers provide their own storage via Repository interfaces.

```
go get github.com/luxfi/captable
```

## Packages

| Package | Purpose |
|---------|---------|
| `pkg/captable` | Cap table CRUD -- companies, share classes, entries, vesting, option grants |
| `pkg/securities` | Securities issuance, transfer, cancellation, conversion with immutable ledger |
| `pkg/stakeholder` | Investor/shareholder management and accreditation tracking |
| `pkg/transfer` | Transfer restrictions -- Rule 144, lockup periods, board approval |
| `pkg/dividend` | Dividend declaration, record date processing, distribution |
| `pkg/corporate` | Corporate actions -- stock splits, mergers, reclassification |
| `pkg/compliance` | Form D, blue sky filings, Reg D compliance checks |
| `pkg/document` | Data rooms, document management, access control, audit trails |
| `pkg/tax` | 1099-DIV, 1099-B, Schedule K-1 generation |

## Usage

```go
import (
    "github.com/luxfi/captable/pkg/captable"
    "github.com/luxfi/captable/pkg/securities"
    "github.com/luxfi/captable/pkg/transfer"
)

// Implement the Repository interface with your database.
repo := myPostgresRepo{}

// Create services.
capSvc := captable.NewService(repo)
secSvc := securities.NewService(secRepo)
xferSvc := transfer.NewService(xferRepo)

// Use them.
err := capSvc.CreateCompany(ctx, &captable.Company{...})
result := xferSvc.CheckRule144(&transfer.Rule144Check{...})
```

## Testing

```bash
go test ./...
```
