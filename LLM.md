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
      iris/        -- IRS IRIS (Information Returns Intake System) e-file adapter
      fire/        -- IRS FIRE (legacy) fixed-width filing adapter (Pub 1220)
      cfsf/        -- Combined Federal/State Filing routing (32+DC participants)
      tcc/         -- Transmitter Control Code registry (per-issuer scoping)
```

## G-15 tax/iris + tax/fire + tax/cfsf + tax/tcc (P0)

`tax.SubmitForm` is the single integration entrypoint — routes to IRIS
by default (mandatory for TY 2024+ per IRS final regs), falls back to
FIRE only for explicitly-flagged legacy-year submissions
(`SubmissionOptions.ForceLegacyFIRE`) or for forms whose TaxYear is
below `IRISMandatoryTaxYear = 2024`.

| Package | Endpoint | Spec |
|---------|----------|------|
| iris | `https://la.www4.irs.gov/iris/a2a` (prod), `https://la.alt.www4.irs.gov/iris/a2a` (AATS) | Pub 5717, Pub 5718 |
| fire | `https://fire.irs.gov` (prod), `https://fire.test.irs.gov` (test) | Pub 1220 |
| cfsf | n/a (routing only) | Pub 1220 Part A §10 |

Form types covered (all 8): 1099-DIV, 1099-B, 1099-INT, 1099-MISC,
1099-NEC, 1099-OID, 1099-K, 1099-R.

CFSF state registry: 32 CFSF participants (AL/AZ/AR/CA/CO/CT/DE/GA/
HI/ID/IN/KS/LA/ME/MD/MA/MI/MN/MS/MO/MT/NE/NJ/NM/NC/ND/OH/OK/SC/WI/DC/
WV) + 19 non-participants (with per-state DOR portal references for
the operator hand-off). `RouteToStates` returns a `routing_type` of
`cfsf` (federal filing satisfies), `direct` (operator must file at
state portal), or `none` (no broad state income tax).

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
