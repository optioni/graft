package lock_test

import (
	"testing"

	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/manifest"
)

func TestCheckPins(t *testing.T) {
	t.Parallel()

	mSource := func(name, rev string) manifest.Source {
		return manifest.Source{
			Name:    name,
			Git:     "github.com/optioni/" + name,
			Rev:     rev,
			Install: []string{"schema:tdd"},
		}
	}
	lSource := func(name, rev string) lock.Source {
		return lock.Source{
			Name:     name,
			Git:      "github.com/optioni/" + name,
			Rev:      rev,
			Resolved: sha,
		}
	}

	for name, tc := range map[string]struct {
		sources []manifest.Source
		lk      *lock.Lock
		want    string
	}{
		"agreeing pins": {
			sources: []manifest.Source{mSource("openspec-schemas", "v1.2.0")},
			lk:      &lock.Lock{Version: 1, Sources: []lock.Source{lSource("openspec-schemas", "v1.2.0")}},
		},
		"moved manifest pin": {
			sources: []manifest.Source{mSource("openspec-schemas", "v1.3.0")},
			lk:      &lock.Lock{Version: 1, Sources: []lock.Source{lSource("openspec-schemas", "v1.2.0")}},
			want:    "graft.toml has rev \"v1.3.0\" for source \"openspec-schemas\" but graft.lock has \"v1.2.0\"; run `graft update` to move the pin",
		},
		"only in the manifest": {
			sources: []manifest.Source{mSource("openspec-schemas", "v1.2.0")},
			lk:      &lock.Lock{Version: 1},
		},
		"only in the lock": {
			lk: &lock.Lock{Version: 1, Sources: []lock.Source{lSource("openspec-schemas", "v1.2.0")}},
		},
		"both empty": {
			lk: &lock.Lock{Version: 1},
		},
		"several sources, the last one moved": {
			sources: []manifest.Source{
				mSource("alpha", "v1.0.0"),
				mSource("openspec-schemas", "v1.3.0"),
			},
			lk: &lock.Lock{Version: 1, Sources: []lock.Source{
				lSource("alpha", "v1.0.0"),
				lSource("openspec-schemas", "v1.2.0"),
			}},
			want: "graft.toml has rev \"v1.3.0\" for source \"openspec-schemas\" but graft.lock has \"v1.2.0\"; run `graft update` to move the pin",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := lock.CheckPins(tc.sources, tc.lk)
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("CheckPins() error = %v, want nil", err)
			case tc.want != "" && err == nil:
				t.Errorf("CheckPins() error = nil, want %q", tc.want)
			case tc.want != "" && err.Error() != tc.want:
				t.Errorf("CheckPins() error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}
