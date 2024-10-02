package services

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

type dummy struct {
	A string `json:"a"`
	B string `json:"b"`
	C uint64 `json:"c"`
}

func Test_ToObjectsMapAndFromObjectsMap(t *testing.T) {
	t.Parallel()

	dummyOriginal := &dummy{
		A: "a",
		B: "b",
		C: 42,
	}

	dummyMap, err := toObjectsMap(dummyOriginal)
	require.Nil(t, err)

	dummyConverted := &dummy{}
	err = fromObjectsMap(dummyMap, dummyConverted)
	require.Nil(t, err)

	require.Equal(t, dummyOriginal, dummyConverted)
}

func Test_IsZeroAmount(t *testing.T) {
	require.True(t, isZeroAmount(""))
	require.True(t, isZeroAmount("0"))
	require.True(t, isZeroAmount("-0"))
	require.True(t, isZeroAmount("00"))
	require.False(t, isZeroAmount("1"))
	require.False(t, isZeroAmount("-1"))
}

func Test_IsNonZeroAmount(t *testing.T) {
	require.False(t, isNonZeroAmount(""))
	require.False(t, isNonZeroAmount("0"))
	require.False(t, isNonZeroAmount("-0"))
	require.False(t, isNonZeroAmount("00"))
	require.True(t, isNonZeroAmount("1"))
	require.True(t, isNonZeroAmount("-1"))
}

func Test_IsZeroBigInt(t *testing.T) {
	require.True(t, isZeroBigIntOrNil(big.NewInt(0)))
	require.True(t, isZeroBigIntOrNil(nil))
	require.False(t, isZeroBigIntOrNil(big.NewInt(42)))
	require.False(t, isZeroBigIntOrNil(big.NewInt(-42)))
}

func Test_GetMagnitudeOfAmount(t *testing.T) {
	require.Equal(t, "100", getMagnitudeOfAmount("100"))
	require.Equal(t, "100", getMagnitudeOfAmount("-100"))
}

func Test_MultiplyUint64(t *testing.T) {
	require.Equal(t, "340282366920938463426481119284349108225", multiplyUint64(math.MaxUint64, math.MaxUint64).String())
	require.Equal(t, "1", multiplyUint64(1, 1).String())
}

func Test_AddBigInt(t *testing.T) {
	require.Equal(t, "12", addBigInt(big.NewInt(7), big.NewInt(5)).String())
}
