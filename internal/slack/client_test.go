package slack

import (
	"testing"
)

func TestMakePermalink(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		timestamp string
		want      string
	}{
		{
			name:      "basic permalink",
			channelID: "C0AB12CD",
			timestamp: "1700000000123456",
			want:      "https://newrelic.slack.com/archives/C0AB12CD/p1700000000123456",
		},
		{
			name:      "different channel",
			channelID: "C1234567",
			timestamp: "1699999999000000",
			want:      "https://newrelic.slack.com/archives/C1234567/p1699999999000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakePermalink(tt.channelID, tt.timestamp)
			if got != tt.want {
				t.Errorf("MakePermalink() = %v, want %v", got, tt.want)
			}
		})
	}
}
