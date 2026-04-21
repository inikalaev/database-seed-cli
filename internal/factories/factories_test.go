package factories

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

func newRng() *rand.Rand {
	return rand.New(rand.NewPCG(42, 0xDEADBEEF))
}

func genCtx(params map[string]any, col seedapi.Column) seedapi.GenContext {
	return seedapi.GenContext{
		Column: col,
		Row:    0,
		Rng:    newRng(),
		Params: seedapi.Params(params),
	}
}

// TestIntegerInclusive pins the round-7 decision that min/max are both
// reachable. A [5,7] range must produce 5, 6, and 7 across many draws.
func TestIntegerInclusive(t *testing.T) {
	seen := map[int]bool{}
	ctx := genCtx(map[string]any{"min": 5, "max": 7}, seedapi.Column{DataType: "integer"})
	for i := 0; i < 500; i++ {
		v := intMech{}.Generate(ctx).(int)
		if v < 5 || v > 7 {
			t.Fatalf("integer out of [5,7]: %d", v)
		}
		seen[v] = true
	}
	for _, want := range []int{5, 6, 7} {
		if !seen[want] {
			t.Errorf("integer never produced %d across 500 draws", want)
		}
	}
}

func TestDurationInclusive(t *testing.T) {
	seen := map[int]bool{}
	ctx := genCtx(map[string]any{"min": 0, "max": 2}, seedapi.Column{DataType: "integer"})
	for i := 0; i < 500; i++ {
		v := durationMech{}.Generate(ctx).(int)
		if v < 0 || v > 2 {
			t.Fatalf("duration out of [0,2]: %d", v)
		}
		seen[v] = true
	}
	if !seen[2] {
		t.Errorf("duration never reached max=2 across 500 draws")
	}
}

func TestFileSizeInclusive(t *testing.T) {
	seen := map[int]bool{}
	ctx := genCtx(map[string]any{"min": 10, "max": 12}, seedapi.Column{DataType: "bigint"})
	for i := 0; i < 500; i++ {
		v := fileSizeMech{}.Generate(ctx).(int)
		if v < 10 || v > 12 {
			t.Fatalf("file_size out of [10,12]: %d", v)
		}
		seen[v] = true
	}
	if !seen[12] {
		t.Errorf("file_size never reached max=12 across 500 draws")
	}
}

// TestIntegerMaxIntBoundary guards against the overflow panic that a naive
// `IntN(hi-lo+1)` triggers when the span saturates. inclusiveIntN must accept
// the full int domain without panicking.
func TestIntegerMaxIntBoundary(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on MaxInt span: %v", r)
		}
	}()
	ctx := genCtx(map[string]any{"min": 0, "max": math.MaxInt}, seedapi.Column{DataType: "bigint"})
	for i := 0; i < 50; i++ {
		v := intMech{}.Generate(ctx).(int)
		if v < 0 {
			t.Fatalf("integer below min on MaxInt span: %d", v)
		}
	}
}

// TestDecimalPrecisionClamp pins the round-7 ULP subtraction: for numeric(5,2)
// the strict PG bound is < 1000; our clamp yields 999.99 as the theoretical
// maximum. Generated values must stay below 1000.
func TestDecimalPrecisionClamp(t *testing.T) {
	precision := 5
	scale := 2
	col := seedapi.Column{
		DataType:     "numeric",
		NumPrecision: &precision,
		NumScale:     &scale,
	}
	ctx := genCtx(map[string]any{"min": 0.0, "max": 1_000_000.0}, col)
	for i := 0; i < 2000; i++ {
		// Each iteration needs a fresh Rng draw — reuse ctx but let its Rng advance.
		v := decimalMech{}.Generate(ctx).(float64)
		if v >= 1000 {
			t.Fatalf("decimal >= 10^(p-s): got %v for numeric(5,2)", v)
		}
		if v < 0 {
			t.Fatalf("decimal below min: got %v", v)
		}
	}
}
