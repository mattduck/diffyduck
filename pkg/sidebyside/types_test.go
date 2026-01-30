package sidebyside

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDaySuffix(t *testing.T) {
	tests := []struct {
		day  int
		want string
	}{
		{1, "st"}, {2, "nd"}, {3, "rd"}, {4, "th"},
		{11, "th"}, {12, "th"}, {13, "th"}, // teens are "th"
		{21, "st"}, {22, "nd"}, {23, "rd"}, {30, "th"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, daySuffix(tt.day), "day %d", tt.day)
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0 seconds ago"},
		{1 * time.Second, "1 second ago"},
		{30 * time.Second, "30 seconds ago"},
		{1 * time.Minute, "1 minute ago"},
		{45 * time.Minute, "45 minutes ago"},
		{1 * time.Hour, "1 hour ago"},
		{5 * time.Hour, "5 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{3 * 24 * time.Hour, "3 days ago"},
		{35 * 24 * time.Hour, "1 month ago"},
		{90 * 24 * time.Hour, "3 months ago"},
		{400 * 24 * time.Hour, "1 year ago"},
		{800 * 24 * time.Hour, "2 years ago"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, relativeTime(tt.d), "duration %v", tt.d)
	}
}

func TestRelativeTime_NegativeDuration(t *testing.T) {
	// Negative durations should be treated as positive.
	assert.Equal(t, "5 hours ago", relativeTime(-5*time.Hour))
}

func TestFormattedDateParts_EmptyDate(t *testing.T) {
	info := CommitInfo{}
	parts := info.FormattedDateParts(time.Now())
	assert.Equal(t, "", parts.Day)
	assert.Equal(t, "details", parts.Date)
	assert.Equal(t, "", parts.Offset)
	assert.Equal(t, "", parts.Ago)
	assert.Equal(t, "details", parts.Plain())
}

func TestFormattedDateParts_UnparseableDate(t *testing.T) {
	info := CommitInfo{Date: "not-a-date"}
	parts := info.FormattedDateParts(time.Now())
	assert.Equal(t, "", parts.Day)
	assert.Equal(t, "not-a-date", parts.Date)
	assert.Equal(t, "", parts.Offset)
	assert.Equal(t, "", parts.Ago)
}

func TestFormattedDateParts_RFC3339(t *testing.T) {
	info := CommitInfo{Date: "2024-01-06T15:03:00+00:00"}
	now := time.Date(2024, 1, 9, 15, 3, 0, 0, time.UTC)
	parts := info.FormattedDateParts(now)

	assert.Equal(t, "Saturday, ", parts.Day)
	assert.Equal(t, "Jan 6th 15:03", parts.Date)
	assert.Equal(t, " +00:00", parts.Offset)
	assert.Equal(t, " (3 days ago)", parts.Ago)
	assert.Equal(t, "Saturday, Jan 6th 15:03 +00:00 (3 days ago)", parts.Plain())
}

func TestFormattedDateParts_NegativeOffset(t *testing.T) {
	info := CommitInfo{Date: "2024-03-21T09:30:00-05:00"}
	now := time.Date(2024, 3, 21, 14, 30, 0, 0, time.UTC)
	parts := info.FormattedDateParts(now)

	assert.Equal(t, "Thursday, ", parts.Day)
	assert.Equal(t, "Mar 21st 09:30", parts.Date)
	assert.Equal(t, " -05:00", parts.Offset)
}

func TestFormattedDateParts_GitDateFormat(t *testing.T) {
	info := CommitInfo{Date: "Mon Jan 15 10:00:00 2024 -0500"}
	now := time.Date(2024, 1, 15, 18, 0, 0, 0, time.UTC)
	parts := info.FormattedDateParts(now)

	assert.Equal(t, "Monday, ", parts.Day)
	assert.Equal(t, "Jan 15th 10:00", parts.Date)
	assert.Equal(t, " -05:00", parts.Offset)
}

func TestFormattedDateParts_PlainDateFormat(t *testing.T) {
	info := CommitInfo{Date: "2024-01-01"}
	now := time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)
	parts := info.FormattedDateParts(now)

	assert.Equal(t, "Monday, ", parts.Day)
	assert.Equal(t, "Jan 1st 00:00", parts.Date)
	assert.Equal(t, " +00:00", parts.Offset)
	assert.Equal(t, " (3 days ago)", parts.Ago)
}

func TestFormattedDate_MatchesPlain(t *testing.T) {
	info := CommitInfo{Date: "2024-01-06T15:03:00+00:00"}
	now := time.Date(2024, 1, 9, 15, 3, 0, 0, time.UTC)
	assert.Equal(t, info.FormattedDateParts(now).Plain(), info.FormattedDate(now))
}

func TestFormattedDateParts_OrdinalSuffixes(t *testing.T) {
	// Verify suffix appears correctly for various days.
	tests := []struct {
		day  int
		want string
	}{
		{1, "1st"}, {2, "2nd"}, {3, "3rd"}, {4, "4th"},
		{11, "11th"}, {12, "12th"}, {13, "13th"},
		{21, "21st"}, {22, "22nd"}, {23, "23rd"},
	}
	now := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		date := time.Date(2024, 1, tt.day, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		info := CommitInfo{Date: date}
		parts := info.FormattedDateParts(now)
		assert.Contains(t, parts.Date, tt.want, "day %d", tt.day)
	}
}
