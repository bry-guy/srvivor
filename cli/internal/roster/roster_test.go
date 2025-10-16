package roster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRoster(t *testing.T) {
	tests := []struct {
		name     string
		season   int
		wantErr  bool
		errMsg   string
		validate func(t *testing.T, roster *SeasonRoster)
	}{
		{
			name:    "load season 48 roster",
			season:  48,
			wantErr: false,
			validate: func(t *testing.T, roster *SeasonRoster) {
				assert.Equal(t, 48, roster.Season)
				assert.Len(t, roster.Contestants, 18)
				// Check some contestants
				assert.Contains(t, roster.Contestants, Contestant{
					CanonicalName: "Kyle",
					FirstName:     "Kyle",
					LastName:      "",
					Nickname:      "",
				})
			},
		},
		{
			name:    "load season 49 roster",
			season:  49,
			wantErr: false,
			validate: func(t *testing.T, roster *SeasonRoster) {
				assert.Equal(t, 49, roster.Season)
				assert.Len(t, roster.Contestants, 18)
				// Check MC with first name Michelle
				found := false
				for _, c := range roster.Contestants {
					if c.CanonicalName == "MC" {
						assert.Equal(t, "Michelle", c.FirstName)
						assert.Equal(t, "MC", c.Nickname)
						found = true
						break
					}
				}
				assert.True(t, found, "MC not found in roster")
			},
		},
		{
			name:    "load non-existent season",
			season:  99,
			wantErr: true,
			errMsg:  "failed to read roster file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roster, err := LoadRoster(tt.season)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, roster)
			if tt.validate != nil {
				tt.validate(t, roster)
			}
		})
	}
}

func TestValidateRoster(t *testing.T) {
	tests := []struct {
		name    string
		roster  SeasonRoster
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid roster",
			roster: SeasonRoster{
				Season: 1,
				Contestants: []Contestant{
					{CanonicalName: "Alice", FirstName: "Alice"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid season",
			roster: SeasonRoster{
				Season: 0,
				Contestants: []Contestant{
					{CanonicalName: "Alice", FirstName: "Alice"},
				},
			},
			wantErr: true,
			errMsg:  "season must be positive",
		},
		{
			name:    "empty contestants",
			roster:  SeasonRoster{Season: 1, Contestants: []Contestant{}},
			wantErr: true,
			errMsg:  "must contain at least one contestant",
		},
		{
			name: "empty canonical name",
			roster: SeasonRoster{
				Season: 1,
				Contestants: []Contestant{
					{CanonicalName: "", FirstName: "Alice"},
				},
			},
			wantErr: true,
			errMsg:  "empty canonical_name",
		},
		{
			name: "empty first name",
			roster: SeasonRoster{
				Season: 1,
				Contestants: []Contestant{
					{CanonicalName: "Alice", FirstName: ""},
				},
			},
			wantErr: true,
			errMsg:  "empty first_name",
		},
		{
			name: "duplicate canonical names",
			roster: SeasonRoster{
				Season: 1,
				Contestants: []Contestant{
					{CanonicalName: "Alice", FirstName: "Alice"},
					{CanonicalName: "Alice", FirstName: "Alice"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate canonical_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRoster(&tt.roster)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
