package sip

const (
	// Informational
	StatusTrying               = 100 // RFC 3261 21.1.1
	StatusRinging              = 180 // RFC 3261 21.1.2
	StatusCallisBeingForwarded = 181 // RFC 3261 21.1.3
	StatusQueued               = 182 // RFC 3261 21.1.4
	StatusSessionProgress      = 183 // RFC 3261 21.1.5

	// Success
	StatusOk = 200 // RFC 3261 21.2.1

	// Redirection
	StatusMultipleChoices    = 300 // RFC 3261 21.3.1
	StatusMovedPermanently   = 301 // RFC 3261 21.3.2
	StatusMovedTemporarily   = 302 // RFC 3261 21.3.3
	StatusUseProxy           = 305 // RFC 3261 21.3.4
	StatusAlternativeService = 380 // RFC 3261 21.3.5

	// Client-Error
	StatusBadRequest                     = 400 // RFC 3261 21.4.1
	StatusUnauthorized                   = 401 // RFC 3261 21.4.2
	StatusPaymentRequired                = 402 // RFC 3261 21.4.3
	StatusForbidden                      = 403 // RFC 3261 21.4.4
	StatusNotFound                       = 404 // RFC 3261 21.4.5
	StatusMethodNotAllowed               = 405 // RFC 3261 21.4.6
	StatusNotAcceptable                  = 406 // RFC 3261 21.4.7
	StatusProxyAuthenticationRequired    = 407 // RFC 3261 21.4.8
	StatusRequestTimeout                 = 408 // RFC 3261 21.4.9
	StatusGone                           = 410 // RFC 3261 21.4.10
	StatusRequestEntityTooLarge          = 413 // RFC 3261 21.4.11
	StatusRequestURITooLarge             = 414 // RFC 3261 21.4.12
	StatusUnsupportedMediaType           = 415 // RFC 3261 21.4.13
	StatusUnsupportedURIScheme           = 416 // RFC 3261 21.4.14
	StatusBadExtension                   = 420 // RFC 3261 21.4.15
	StatusExtensionRequired              = 421 // RFC 3261 21.4.16
	StatusIntervalTooBrief               = 423 // RFC 3261 21.4.17
	StatusTemporarilynotavailable        = 480 // RFC 3261 21.4.18
	StatusCallLegTransactionDoesNotExist = 481 // RFC 3261 21.4.19
	StatusLoopDetected                   = 482 // RFC 3261 21.4.20
	StatusTooManyHops                    = 483 // RFC 3261 21.4.21
	StatusAddressIncomplete              = 484 // RFC 3261 21.4.22
	StatusAmbiguous                      = 485 // RFC 3261 21.4.23
	StatusBusyHere                       = 486 // RFC 3261 21.4.24
	StatusRequestTerminated              = 487 // RFC 3261 21.4.25
	StatusNotAcceptableHere              = 488 // RFC 3261 21.4.26
	StatusRequestPending                 = 491 // RFC 3261 21.4.27
	StatusUndecipherable                 = 493 // RFC 3261 21.4.28

	// Server-Error
	StatusInternalServerError    = 500 // RFC 3261 21.5.1
	StatusNotImplemented         = 501 // RFC 3261 21.5.2
	StatusBadGateway             = 502 // RFC 3261 21.5.3
	StatusServiceUnavailable     = 503 // RFC 3261 21.5.4
	StatusServerTimeout          = 504 // RFC 3261 21.5.5
	StatusSIPVersionnotsupported = 505 // RFC 3261 21.5.6
	StatusMessageTooLarge        = 513 // RFC 3261 21.5.7

	// Global-Failure
	StatusBusyEverywhere       = 600 // RFC 3261 21.6.1
	StatusDecline              = 603 // RFC 3261 21.6.2
	StatusDoesnotexistanywhere = 604 // RFC 3261 21.6.3
	StatusGlobalNotAcceptable  = 606 // RFC 3261 21.6.4
)

var statusText = map[int]string{
	StatusTrying:               "Trying",
	StatusRinging:              "Ringing",
	StatusCallisBeingForwarded: "Call Is Being Forwarded",
	StatusQueued:               "Queued",
	StatusSessionProgress:      "Session Progress",

	StatusOk: "OK",

	StatusMultipleChoices:    "Multiple Choices",
	StatusMovedPermanently:   "Moved Permanently",
	StatusMovedTemporarily:   "Moved Temporarily",
	StatusUseProxy:           "Use Proxy",
	StatusAlternativeService: "Alternative Service",

	StatusBadRequest:                     "Bad Request",
	StatusUnauthorized:                   "Unauthorized",
	StatusPaymentRequired:                "Payment Required",
	StatusForbidden:                      "Forbidden",
	StatusNotFound:                       "Not Found",
	StatusMethodNotAllowed:               "Method Not Allowed",
	StatusNotAcceptable:                  "Not Acceptable",
	StatusProxyAuthenticationRequired:    "Proxy Authentication Required",
	StatusRequestTimeout:                 "Request Timeout",
	StatusGone:                           "Gone",
	StatusRequestEntityTooLarge:          "Request Entity Too Large",
	StatusRequestURITooLarge:             "Request-URI Too Large",
	StatusUnsupportedMediaType:           "Unsupported Media Type",
	StatusUnsupportedURIScheme:           "Unsupported URI Scheme",
	StatusBadExtension:                   "Bad Extension",
	StatusExtensionRequired:              "Extension Required",
	StatusIntervalTooBrief:               "Interval Too Brief",
	StatusTemporarilynotavailable:        "Temporarily not available",
	StatusCallLegTransactionDoesNotExist: "Call Leg/Transaction Does Not Exist",
	StatusLoopDetected:                   "Loop Detected",
	StatusTooManyHops:                    "Too Many Hops",
	StatusAddressIncomplete:              "Address Incomplete",
	StatusAmbiguous:                      "Ambiguous",
	StatusBusyHere:                       "Busy Here",
	StatusRequestTerminated:              "Request Terminated",
	StatusNotAcceptableHere:              "Not Acceptable Here",
	StatusRequestPending:                 "Request Pending",
	StatusUndecipherable:                 "Undecipherable",

	StatusInternalServerError:    "Internal Server Error",
	StatusNotImplemented:         "Not Implemented",
	StatusBadGateway:             "Bad Gateway",
	StatusServiceUnavailable:     "Service Unavailable",
	StatusServerTimeout:          "Server Time-out",
	StatusSIPVersionnotsupported: "SIP Version not supported",
	StatusMessageTooLarge:        "Message Too Large",

	StatusBusyEverywhere:       "Busy Everywhere",
	StatusDecline:              "Decline",
	StatusDoesnotexistanywhere: "Does not exist anywhere",
	StatusGlobalNotAcceptable:  "Not Acceptable",
}

var (
	ErrStatusError = &ProtocolError{"SIP Protocol Status Error"}
)
