package providers_test

import (
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm/providers"
)

func TestFindUsesSuppliedProviders(t *testing.T) {
	want := providers.Default()[0]
	want.Name = "custom"
	want.BaseURL = "https://custom.example/v1"
	want.DefaultModel = "custom-model"
	want.APIKeyEnvironment = "CUSTOM_API_KEY"
	available := []providers.Provider{{Name: "unconfigured"}, want}
	got, err := providers.Find(available, "custom")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.BaseURL != want.BaseURL || got.DefaultModel != want.DefaultModel || got.APIKeyEnvironment != want.APIKeyEnvironment || got.NewClient == nil {
		t.Fatalf("Find returned incorrect provider: %+v", got)
	}
}

func TestFindErrors(t *testing.T) {
	for _, test := range []struct {
		name      string
		available []providers.Provider
		requested string
		want      string
	}{
		{
			name: "unknown provider", available: []providers.Provider{{Name: "second"}, {Name: "first"}}, requested: "unknown",
			want: `unsupported provider "unknown"; available providers: second, first`,
		},
		{
			name: "no providers", requested: "unknown",
			want: `unsupported provider "unknown"; available providers: `,
		},
		{
			name: "missing factory", available: []providers.Provider{{Name: "unconfigured"}}, requested: "unconfigured",
			want: `provider "unconfigured" has no client factory`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := providers.Find(test.available, test.requested)
			if err == nil || err.Error() != test.want {
				t.Fatalf("Find error = %v, want %q", err, test.want)
			}
		})
	}
}
