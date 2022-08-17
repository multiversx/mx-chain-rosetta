package provider

import (
	"errors"
	"testing"

	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

type dummyResourceApiResponse struct {
	Foo   string `json:"foo"`
	Bar   string `json:"bar"`
	Error string `json:"error"`
}

func (response *dummyResourceApiResponse) GetErrorMessage() string {
	return response.Error
}

func TestNetworkProvider_GetResource(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	t.Run("with success", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = struct {
			Foo string `json:"foo"`
			Bar string `json:"bar"`
		}{
			Foo: "foo",
			Bar: "bar",
		}

		response := &dummyResourceApiResponse{}
		err = provider.getResource("/test", response)
		require.Nil(t, err)
		require.Equal(t, "foo", response.Foo)
		require.Equal(t, "bar", response.Bar)
	})

	t.Run("with error (as err, unstructured)", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("arbitrary error")
		observerFacade.MockGetResponse = nil

		response := &dummyResourceApiResponse{}
		err = provider.getResource("/test", response)
		require.Equal(t, err, errors.New("arbitrary error"))
		require.Equal(t, &dummyResourceApiResponse{}, response)
	})

	t.Run("with error (as err, structured)", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("{ \"error\": \"internal error\", \"code\": \"err\" }")
		observerFacade.MockGetResponse = nil

		response := &dummyResourceApiResponse{}
		err = provider.getResource("/test", response)
		require.Equal(t, errors.New("internal error: err"), err)
		require.Equal(t, &dummyResourceApiResponse{}, response)
	})

	t.Run("with error (as field on payload)", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = struct {
			Foo   string `json:"foo"`
			Error string `json:"error"`
		}{
			Error: "error on payload",
		}

		response := &dummyResourceApiResponse{}
		err = provider.getResource("/test", response)
		require.Equal(t, err, errors.New("error on payload"))
		require.Equal(t, &dummyResourceApiResponse{Error: "error on payload"}, response)
	})
}
