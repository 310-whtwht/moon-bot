package indicator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEMA(t *testing.T) {
	e := NewEMA(3) // k = 0.5
	for i, v := range []float64{1, 2} {
		_, ok := e.Update(v)
		assert.False(t, ok, "value %d", i)
	}

	got, ok := e.Update(3) // seed = SMA(1,2,3) = 2
	assert.True(t, ok)
	assert.InDelta(t, 2.0, got, 1e-12)

	got, _ = e.Update(4) // 2 + 0.5*(4-2) = 3
	assert.InDelta(t, 3.0, got, 1e-12)

	got, _ = e.Update(10) // 3 + 0.5*(10-3) = 6.5
	assert.InDelta(t, 6.5, got, 1e-12)

	v, ok := e.Value()
	assert.True(t, ok)
	assert.InDelta(t, 6.5, v, 1e-12)
}

func TestATR(t *testing.T) {
	a := NewATR(2)

	// TR1 = high-low = 2 (no previous close)
	_, ok := a.Update(11, 9, 10)
	assert.False(t, ok)

	// TR2 = max(12-10.5=1.5, |12-10|=2, |10.5-10|=0.5) = 2 → seed = (2+2)/2 = 2
	got, ok := a.Update(12, 10.5, 11)
	assert.True(t, ok)
	assert.InDelta(t, 2.0, got, 1e-12)

	// Gap down: TR3 = max(9-8=1, |9-11|=2, |8-11|=3) = 3 → (2*1 + 3)/2 = 2.5
	got, _ = a.Update(9, 8, 8.5)
	assert.InDelta(t, 2.5, got, 1e-12)
}
