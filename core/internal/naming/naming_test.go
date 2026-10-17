package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsGenericTitle(t *testing.T) {
	for _, s := range []string{"", "  ", "BaseMap_REA", "basemap", "Base Map", "Layer 1", "Untitled", "Sem título", "Map"} {
		t.Run("generic "+s, func(t *testing.T) {
			// Act
			got := IsGenericTitle(s)

			// Assert
			assert.True(t, got)
		})
	}
	for _, s := range []string{"CCV REA XP2-SÃO PAULO", "São Paulo", "ILS X RWY 17R", "Mapa Aeronáutico de Curitiba"} {
		t.Run("meaningful "+s, func(t *testing.T) {
			// Act
			got := IsGenericTitle(s)

			// Assert
			assert.False(t, got)
		})
	}
}
