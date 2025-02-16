package utils

import (
    "net/http"
    "time"
)

func NewHTTPClient(timeout time.Duration) *http.Client {
    return &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            ResponseHeaderTimeout: timeout * 2 / 3,
            TLSHandshakeTimeout:  timeout / 3,
            DisableKeepAlives:    true,
            MaxIdleConns:         100,
            MaxIdleConnsPerHost:  100,
        },
    }
}
