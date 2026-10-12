package naming

import "testing"

func TestIsGenericTitle(t *testing.T) {
	for _, s := range []string{"", "  ", "BaseMap_REA", "basemap", "Base Map", "Layer 1", "Untitled", "Sem título", "Map"} {
		if !IsGenericTitle(s) {
			t.Errorf("%q should be generic", s)
		}
	}
	for _, s := range []string{"CCV REA XP2-SÃO PAULO", "São Paulo", "ILS X RWY 17R", "Mapa Aeronáutico de Curitiba"} {
		if IsGenericTitle(s) {
			t.Errorf("%q should not be generic", s)
		}
	}
}
