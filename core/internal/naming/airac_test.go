package naming

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCycleOfAnEffectiveDate(t *testing.T) {
	// dates of published charts, from three countries' calendars: all the same AIRAC grid
	cases := map[string]string{
		"2026-01-22": "2601", "2026-08-06": "2608", "2026-10-01": "2610", "2026-10-29": "2611",
		"2026-12-24": "2613", "2025-08-07": "2508", "2024-11-28": "2412", "2024-12-26": "2413",
		"2027-01-21": "2701",
		// not the first day of a cycle: no cycle
		"2026-05-20": "", "2026-10-02": "", "nonsense": "",
	}
	for date, want := range cases {
		t.Run(date, func(t *testing.T) {
			// Act
			got := CycleOfDate(date)

			// Assert
			assert.Equal(t, want, got)
		})
	}
}

func TestDateOfCycleIsTheInverse(t *testing.T) {
	for _, c := range []string{"2601", "2608", "2610", "2613", "2508", "2412", "2413", "2701"} {
		t.Run(c, func(t *testing.T) {
			// Act
			d, ok := DateOfCycle(c)

			// Assert
			assert.True(t, ok)
			assert.Equal(t, c, CycleOfDate(d), "does not come back")
		})
	}
}

func TestDateOfCycleRefusesWhatIsNotACycle(t *testing.T) {
	for _, bad := range []string{"", "26", "2600", "2699", "abcd", "2614x"} {
		t.Run(bad, func(t *testing.T) {
			// Act
			_, ok := DateOfCycle(bad)

			// Assert
			assert.False(t, ok)
		})
	}
}

func TestAIRACInForce(t *testing.T) {
	// Arrange
	on := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	// Act
	eff, id := AIRAC(on)

	// Assert
	assert.Equal(t, "2610", id)
	assert.Equal(t, "2026-10-01", eff.Format("2006-01-02"))
}

func TestNextAIRAC(t *testing.T) {
	// Arrange
	on := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	// Act
	eff, id := NextAIRAC(on)

	// Assert
	assert.Equal(t, "2611", id)
	assert.Equal(t, "2026-10-29", eff.Format("2006-01-02"))
}
