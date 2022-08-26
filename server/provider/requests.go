package provider

import "errors"

func (provider *networkProvider) getResource(url string, response resourceApiResponseHandler) error {
	if provider.isOffline {
		return errIsOffline
	}

	err := provider.getResourceWithErrConversion(url, response)
	if err != nil {
		log.Warn("getResource()", "url", url, "err", err)
		return err
	}

	return nil
}

func (provider *networkProvider) getResourceWithErrConversion(url string, response resourceApiResponseHandler) error {
	_, err := provider.observerFacade.CallGetRestEndPoint(provider.observerUrl, url, response)
	if err != nil {
		return convertStructuredApiErrToFlatErr(err)
	}
	if response.GetErrorMessage() != "" {
		return errors.New(response.GetErrorMessage())
	}

	return nil
}
