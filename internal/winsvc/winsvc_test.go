package winsvc

import "testing"

func TestWithDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   Options
		want Options
	}{
		{
			name: "nothing set",
			want: Options{Name: DefaultName, DisplayName: DefaultDisplayName, Description: DefaultDescription},
		},
		{
			name: "name only: display name follows it",
			in:   Options{Name: "ProxySrv"},
			want: Options{Name: "ProxySrv", DisplayName: "ProxySrv", Description: DefaultDescription},
		},
		{
			name: "all set, trimmed, nothing appended",
			in:   Options{Name: " ProxySrv ", DisplayName: " Proxy ", Description: " Forwards requests "},
			want: Options{Name: "ProxySrv", DisplayName: "Proxy", Description: "Forwards requests"},
		},
		{
			name: "display name without name keeps the default name",
			in:   Options{DisplayName: "Proxy"},
			want: Options{Name: DefaultName, DisplayName: "Proxy", Description: DefaultDescription},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.WithDefaults(); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      Options
		wantErr bool
	}{
		{name: "default", in: Options{}.WithDefaults()},
		{name: "spaces are fine", in: Options{Name: "Elvare Proxy", DisplayName: "x"}},
		{name: "empty", in: Options{}, wantErr: true},
		{name: "backslash", in: Options{Name: `a\b`, DisplayName: "x"}, wantErr: true},
		{name: "slash", in: Options{Name: "a/b", DisplayName: "x"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
