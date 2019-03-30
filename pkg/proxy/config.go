package proxy

import "net/url"

type RequestConfigProvider interface {
	GetRequestConfig(requestURI []byte) RequestConfig
}

type RequestConfig struct {
	Schema              *[]byte
	BackendURL          url.URL
	AddHeadersToContext [][]byte
}

type StaticRequestConfigProvider struct {
	config RequestConfig
}

func (s *StaticRequestConfigProvider) GetRequestConfig(requestURI []byte) RequestConfig {
	return s.config
}

func NewStaticRequestConfigProvider(config RequestConfig) *StaticRequestConfigProvider {
	return &StaticRequestConfigProvider{
		config: config,
	}
}