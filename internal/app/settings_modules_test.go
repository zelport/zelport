package app

import (
	"github.com/zelport/zelport/internal/feature"
	"testing"
)

type placedModule struct {
	testModule
	placement, tab string
}

func (m *placedModule) Descriptor() feature.Descriptor {
	return feature.Descriptor{Maturity: feature.Stable, ID: m.id, Name: m.id, TabID: m.tab, UIPlacement: m.placement}
}
func TestSettingsModuleDescriptors(t *testing.T) {
	_, err := New("", 8080, &placedModule{testModule: testModule{id: "one"}, placement: "settings"}, &placedModule{testModule: testModule{id: "two"}, placement: "settings"})
	if err != nil {
		t.Fatal("independent settings sections rejected", err)
	}
	for _, v := range []struct{ placement, tab string }{{"", "settings"}, {"", "dashboard"}, {"", ""}, {"settings", "relay"}, {"invalid", "relay"}} {
		_, err := New("", 8080, &placedModule{testModule: testModule{id: "one"}, placement: v.placement, tab: v.tab})
		if err == nil {
			t.Fatal("invalid descriptor accepted", v)
		}
	}
}
