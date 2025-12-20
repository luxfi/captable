# LLM.md - Lux Cap Table

## Overview
Go module: `github.com/luxfi/captable`

Core Transfer Agent (TA) engine. Reusable library -- no database, no HTTP, no framework.
Consumers (e.g., `~/work/liquidity/ta/`) import this and provide their own Repository implementations.

## Build & Test
```bash
go build ./...
go test ./...
```

## Structure
```
captable/
  go.mod
  pkg/
    captable/      -- Company, ShareClass, Entry CRUD; cap table summary
    securities/    -- Security issuance/transfer/cancel/convert + immutable ledger
    stakeholder/   -- Investor management, accreditation checks
    transfer/      -- Transfer restrictions: Rule 144, lockup, board approval
    dividend/      -- Dividend declaration, record date, distribution
    corporate/     -- Corporate actions: splits, mergers, reclassification
    compliance/    -- Form D, blue sky, Reg D compliance checks
    document/      -- Data rooms, document metadata, access control, audit
    tax/           -- 1099-DIV, 1099-B, K-1 generation
```

## Pattern
Each package follows the same structure:
- `types.go` -- domain types (structs with JSON tags)
- `repository.go` -- storage interface (consumers implement)
- `service.go` -- business logic (struct with methods, not Go interface)
- `service_test.go` -- tests using in-memory repository

No database dependency. No external dependencies. Standard library only.

## Key Design Decisions
- Repository pattern: all storage injected, no SQL/ORM in this module
- Services are concrete structs, not interfaces (matches broker pattern)
- In-memory repos in test files for testing without infrastructure
- Validation at service boundary, trust internal functions
- Immutable ledger in securities package for audit trail
- HoldingsProvider/DividendProvider interfaces for cross-package data flow

## How ta/ imports this
```go
// In ~/work/liquidity/ta/go.mod:
require github.com/luxfi/captable v1.0.0

// In ta code:
import "github.com/luxfi/captable/pkg/captable"
svc := captable.NewService(postgresRepo)
```
