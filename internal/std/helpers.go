package std

import (
	"errors"
	"math/big"
	mrand "math/rand"
	"sort"
	"strconv"
	"time"

	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
)

func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func isRangeError(err error) bool {
	return errors.Is(err, strconv.ErrRange)
}

func stableSort(idx []int, less func(x, y int) bool) {
	sort.SliceStable(idx, func(a, b int) bool { return less(idx[a], idx[b]) })
}

func cmpLess(a, b val.Value) (bool, error) {
	return pyops.Less(a, b)
}

func keyStr(v val.Value) string {
	switch x := v.(type) {
	case val.Str:
		return string(x)
	}
	return pyops.PyStr(v)
}

type randSrc struct{ rng *mrand.Rand }

func newRandomSource() *randSrc {
	return &randSrc{rng: mrand.New(mrand.NewSource(time.Now().UnixNano()))}
}

func (r *randSrc) random() float64 { return r.rng.Float64() }

func (r *randSrc) randint(a, b *big.Int) *big.Int {
	lo := a
	hi := b
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	scope := new(big.Int).Sub(hi, lo)
	scope.Add(scope, big.NewInt(1))
	if scope.IsInt64() {
		return lo.Add(lo, big.NewInt(r.rng.Int63n(scope.Int64())))
	}

	bits := uint(scope.BitLen())
	n := new(big.Int)
	for {
		n.Rand(r.rng, new(big.Int).Lsh(big.NewInt(1), bits))
		if n.Cmp(scope) < 0 {
			return lo.Add(lo, n)
		}
	}
}
