package roster

// Contestant represents a contestant in a Survivor season
type Contestant struct {
	CanonicalName string `json:"canonical_name"` // The official name used in drafts (e.g., "Sophie S")
	FirstName     string `json:"first_name"`     // First name (e.g., "Sophie")
	LastName      string `json:"last_name"`      // Last name (e.g., "Stevens")
	Nickname      string `json:"nickname"`       // Preferred nickname if any (e.g., "MC" for Michelle)
}

// SeasonRoster represents the roster for a specific season
type SeasonRoster struct {
	Season      int          `json:"season"`
	Contestants []Contestant `json:"contestants"`
}
