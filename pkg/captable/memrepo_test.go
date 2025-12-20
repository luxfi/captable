package captable

import (
	"context"
	"fmt"
	"sync"
)

// memRepo is an in-memory Repository for testing.
type memRepo struct {
	mu             sync.RWMutex
	companies      map[string]*Company
	shareClasses   map[string]*ShareClass
	entries        map[string]*Entry
	vesting        map[string]*VestingSchedule
	optionGrants   map[string]*OptionGrant
	equityPlans    map[string]*EquityPlan
}

func newMemRepo() *memRepo {
	return &memRepo{
		companies:    make(map[string]*Company),
		shareClasses: make(map[string]*ShareClass),
		entries:      make(map[string]*Entry),
		vesting:      make(map[string]*VestingSchedule),
		optionGrants: make(map[string]*OptionGrant),
		equityPlans:  make(map[string]*EquityPlan),
	}
}

func (r *memRepo) CreateCompany(_ context.Context, c *Company) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.companies[c.ID]; exists {
		return fmt.Errorf("company %s already exists", c.ID)
	}
	cp := *c
	r.companies[c.ID] = &cp
	return nil
}

func (r *memRepo) GetCompany(_ context.Context, id string) (*Company, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.companies[id]
	if !ok {
		return nil, fmt.Errorf("company %s not found", id)
	}
	cp := *c
	return &cp, nil
}

func (r *memRepo) UpdateCompany(_ context.Context, c *Company) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.companies[c.ID]; !exists {
		return fmt.Errorf("company %s not found", c.ID)
	}
	cp := *c
	r.companies[c.ID] = &cp
	return nil
}

func (r *memRepo) ListCompanies(_ context.Context, tenantID string, _ ListParams) ([]*Company, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Company
	for _, c := range r.companies {
		if c.TenantID == tenantID {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateShareClass(_ context.Context, sc *ShareClass) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.shareClasses[sc.ID]; exists {
		return fmt.Errorf("share class %s already exists", sc.ID)
	}
	cp := *sc
	r.shareClasses[sc.ID] = &cp
	return nil
}

func (r *memRepo) GetShareClass(_ context.Context, id string) (*ShareClass, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sc, ok := r.shareClasses[id]
	if !ok {
		return nil, fmt.Errorf("share class %s not found", id)
	}
	cp := *sc
	return &cp, nil
}

func (r *memRepo) UpdateShareClass(_ context.Context, sc *ShareClass) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.shareClasses[sc.ID]; !exists {
		return fmt.Errorf("share class %s not found", sc.ID)
	}
	cp := *sc
	r.shareClasses[sc.ID] = &cp
	return nil
}

func (r *memRepo) ListShareClasses(_ context.Context, companyID string) ([]*ShareClass, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*ShareClass
	for _, sc := range r.shareClasses {
		if sc.CompanyID == companyID {
			cp := *sc
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateEntry(_ context.Context, e *Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[e.ID]; exists {
		return fmt.Errorf("entry %s already exists", e.ID)
	}
	cp := *e
	r.entries[e.ID] = &cp
	return nil
}

func (r *memRepo) GetEntry(_ context.Context, id string) (*Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok {
		return nil, fmt.Errorf("entry %s not found", id)
	}
	cp := *e
	return &cp, nil
}

func (r *memRepo) UpdateEntry(_ context.Context, e *Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[e.ID]; !exists {
		return fmt.Errorf("entry %s not found", e.ID)
	}
	cp := *e
	r.entries[e.ID] = &cp
	return nil
}

func (r *memRepo) ListEntries(_ context.Context, companyID string, _ ListParams) ([]*Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Entry
	for _, e := range r.entries {
		if e.CompanyID == companyID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) ListEntriesByStakeholder(_ context.Context, companyID, stakeholderID string) ([]*Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Entry
	for _, e := range r.entries {
		if e.CompanyID == companyID && e.StakeholderID == stakeholderID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateVestingSchedule(_ context.Context, v *VestingSchedule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *v
	r.vesting[v.ID] = &cp
	return nil
}

func (r *memRepo) GetVestingSchedule(_ context.Context, id string) (*VestingSchedule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.vesting[id]
	if !ok {
		return nil, fmt.Errorf("vesting schedule %s not found", id)
	}
	cp := *v
	return &cp, nil
}

func (r *memRepo) UpdateVestingSchedule(_ context.Context, v *VestingSchedule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *v
	r.vesting[v.ID] = &cp
	return nil
}

func (r *memRepo) CreateOptionGrant(_ context.Context, og *OptionGrant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *og
	r.optionGrants[og.ID] = &cp
	return nil
}

func (r *memRepo) GetOptionGrant(_ context.Context, id string) (*OptionGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	og, ok := r.optionGrants[id]
	if !ok {
		return nil, fmt.Errorf("option grant %s not found", id)
	}
	cp := *og
	return &cp, nil
}

func (r *memRepo) UpdateOptionGrant(_ context.Context, og *OptionGrant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *og
	r.optionGrants[og.ID] = &cp
	return nil
}

func (r *memRepo) ListOptionGrants(_ context.Context, companyID string, _ ListParams) ([]*OptionGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*OptionGrant
	for _, og := range r.optionGrants {
		if og.CompanyID == companyID {
			cp := *og
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateEquityPlan(_ context.Context, ep *EquityPlan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *ep
	r.equityPlans[ep.ID] = &cp
	return nil
}

func (r *memRepo) GetEquityPlan(_ context.Context, id string) (*EquityPlan, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ep, ok := r.equityPlans[id]
	if !ok {
		return nil, fmt.Errorf("equity plan %s not found", id)
	}
	cp := *ep
	return &cp, nil
}

func (r *memRepo) UpdateEquityPlan(_ context.Context, ep *EquityPlan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *ep
	r.equityPlans[ep.ID] = &cp
	return nil
}

func (r *memRepo) ListEquityPlans(_ context.Context, companyID string) ([]*EquityPlan, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*EquityPlan
	for _, ep := range r.equityPlans {
		if ep.CompanyID == companyID {
			cp := *ep
			out = append(out, &cp)
		}
	}
	return out, nil
}
