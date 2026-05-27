// CFSF state participation registry per IRS Publication 1220 Part A
// §10 — current as of TY 2024. The IRS publishes the participant list
// annually; this registry is the canonical Lux mirror.
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 1220 Part A §10 — Combined Federal/State Filing Program
// Source-ref: https://www.irs.gov/pub/irs-pdf/p1220.pdf
package cfsf

// stateRegistry is the static CFSF participation table. The IRS
// publishes the TY 2024 list at Pub 1220 Part A §10 Table 1 — 32
// participating states + DC. Non-CFSF jurisdictions are listed with
// FallbackPortal set to the state's DOR portal (where known).
//
// Where the participation status is uncertain (the IRS occasionally
// adds / removes states between TYs) the operator should re-verify
// against the current Publication 1220 before relying on this table.
var stateRegistry = map[State]StateProfile{
	// --- CFSF participants (TY 2024, per Pub 1220 Part A §10 Table 1) ---
	"AL": {State: "AL", ParticipatesInCFSF: true, CFSFCode: "01"},
	"AZ": {State: "AZ", ParticipatesInCFSF: true, CFSFCode: "04"},
	"AR": {State: "AR", ParticipatesInCFSF: true, CFSFCode: "05"},
	"CA": {State: "CA", ParticipatesInCFSF: true, CFSFCode: "06"},
	"CO": {State: "CO", ParticipatesInCFSF: true, CFSFCode: "07"},
	"CT": {State: "CT", ParticipatesInCFSF: true, CFSFCode: "08"},
	"DE": {State: "DE", ParticipatesInCFSF: true, CFSFCode: "10"},
	"GA": {State: "GA", ParticipatesInCFSF: true, CFSFCode: "13"},
	"HI": {State: "HI", ParticipatesInCFSF: true, CFSFCode: "15"},
	"ID": {State: "ID", ParticipatesInCFSF: true, CFSFCode: "16"},
	"IN": {State: "IN", ParticipatesInCFSF: true, CFSFCode: "18"},
	"KS": {State: "KS", ParticipatesInCFSF: true, CFSFCode: "20"},
	"LA": {State: "LA", ParticipatesInCFSF: true, CFSFCode: "22"},
	"ME": {State: "ME", ParticipatesInCFSF: true, CFSFCode: "23"},
	"MD": {State: "MD", ParticipatesInCFSF: true, CFSFCode: "24"},
	"MA": {State: "MA", ParticipatesInCFSF: true, CFSFCode: "25"},
	"MI": {State: "MI", ParticipatesInCFSF: true, CFSFCode: "26"},
	"MN": {State: "MN", ParticipatesInCFSF: true, CFSFCode: "27"},
	"MS": {State: "MS", ParticipatesInCFSF: true, CFSFCode: "28"},
	"MO": {State: "MO", ParticipatesInCFSF: true, CFSFCode: "29"},
	"MT": {State: "MT", ParticipatesInCFSF: true, CFSFCode: "30"},
	"NE": {State: "NE", ParticipatesInCFSF: true, CFSFCode: "31"},
	"NJ": {State: "NJ", ParticipatesInCFSF: true, CFSFCode: "34"},
	"NM": {State: "NM", ParticipatesInCFSF: true, CFSFCode: "35"},
	"NC": {State: "NC", ParticipatesInCFSF: true, CFSFCode: "37"},
	"ND": {State: "ND", ParticipatesInCFSF: true, CFSFCode: "38"},
	"OH": {State: "OH", ParticipatesInCFSF: true, CFSFCode: "39"},
	"OK": {State: "OK", ParticipatesInCFSF: true, CFSFCode: "40"},
	"SC": {State: "SC", ParticipatesInCFSF: true, CFSFCode: "45"},
	"WI": {State: "WI", ParticipatesInCFSF: true, CFSFCode: "55"},

	// --- DC + WV (also publish via CFSF historically; verify TY 2024) ---
	"DC": {State: "DC", ParticipatesInCFSF: true, CFSFCode: "11"},
	"WV": {State: "WV", ParticipatesInCFSF: true, CFSFCode: "54"},

	// --- Non-CFSF states (require direct state filing) ---
	"AK": {State: "AK", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — Alaska has no state income tax"},
	"FL": {State: "FL", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — Florida has no state income tax"},
	"IL": {State: "IL", ParticipatesInCFSF: false, FallbackPortal: PortalFSET, FallbackURL: "https://mytax.illinois.gov/", FormThreshold: 10},
	"IA": {State: "IA", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://tax.iowa.gov/efile-pay/iowa-1099-information-returns", FormThreshold: 10},
	"KY": {State: "KY", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://onestop.ky.gov/", FormThreshold: 26},
	"NH": {State: "NH", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — New Hampshire has no broad state income tax"},
	"NV": {State: "NV", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — Nevada has no state income tax"},
	"NY": {State: "NY", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://www.tax.ny.gov/bus/efile/elf_business_pit.htm", FormThreshold: 10},
	"OR": {State: "OR", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://revenueonline.dor.oregon.gov/", FormThreshold: 10},
	"PA": {State: "PA", ParticipatesInCFSF: false, FallbackPortal: PortalFSET, FallbackURL: "https://mypath.pa.gov/", FormThreshold: 10},
	"RI": {State: "RI", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://taxportal.ri.gov/", FormThreshold: 25},
	"SD": {State: "SD", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — South Dakota has no state income tax"},
	"TN": {State: "TN", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — Tennessee has no broad state income tax"},
	"TX": {State: "TX", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — Texas has no state income tax"},
	"UT": {State: "UT", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://tap.utah.gov/", FormThreshold: 250},
	"VT": {State: "VT", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://myvtax.vermont.gov/", FormThreshold: 25},
	"VA": {State: "VA", ParticipatesInCFSF: false, FallbackPortal: PortalDORWebsite, FallbackURL: "https://www.business.tax.virginia.gov/", FormThreshold: 10},
	"WA": {State: "WA", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — Washington has no state income tax"},
	"WY": {State: "WY", ParticipatesInCFSF: false, FallbackPortal: PortalNone, FallbackURL: "n/a — Wyoming has no state income tax"},
}

// GetProfile returns the StateProfile for a USPS state code. The
// second return is false if the state is unknown (e.g., a US territory
// not yet in the registry).
func GetProfile(s State) (StateProfile, bool) {
	p, ok := stateRegistry[s]
	return p, ok
}

// AllParticipants returns the slice of CFSF-participating states in
// sorted-by-code order. Useful for operator reporting.
func AllParticipants() []StateProfile {
	out := make([]StateProfile, 0)
	for _, p := range stateRegistry {
		if p.ParticipatesInCFSF {
			out = append(out, p)
		}
	}
	return out
}

// AllNonParticipants returns the slice of non-CFSF states known to
// the registry. Useful to drive operator awareness of states the
// federal filing does not automatically satisfy.
func AllNonParticipants() []StateProfile {
	out := make([]StateProfile, 0)
	for _, p := range stateRegistry {
		if !p.ParticipatesInCFSF {
			out = append(out, p)
		}
	}
	return out
}

// RegisteredStateCount returns (cfsfCount, nonCfsfCount, total).
func RegisteredStateCount() (int, int, int) {
	cfsf, non := 0, 0
	for _, p := range stateRegistry {
		if p.ParticipatesInCFSF {
			cfsf++
		} else {
			non++
		}
	}
	return cfsf, non, cfsf + non
}
