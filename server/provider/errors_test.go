package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertStructuredApiErrToFlatErr(t *testing.T) {
	err := convertStructuredApiErrToFlatErr(errors.New("{\"data\":null,\"error\":\"too many requests\",\"code\":\"system_busy\"}"))
	require.Equal(t, errors.New("too many requests: system_busy"), err)

	err = convertStructuredApiErrToFlatErr(errors.New("this is not a structured error"))
	require.Equal(t, errors.New("this is not a structured error"), err)
}
