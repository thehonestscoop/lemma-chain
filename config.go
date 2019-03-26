// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

// hashIDSalt is used to convert uids to hashids. Once set in production, do not modify
var hashIDSalt = lookupEnvOrUseDefault("HASHID_SALT", "ffb80dba55db4b7ab49cb83ed96eca29")

// maxDataPayload is used to set the maximum payload size in kB for a new node's custom data
var maxDataPayload = lookupEnvOrUseDefaultInt("MAX_PAYLOAD_KB", 12)

// stdQueryTimeout is used to set the maximum query duration (in ms) for potentially
// expensive GET requests.
var stdQueryTimeout = lookupEnvOrUseDefaultInt64("QUERY_TIMEOUT", 300)

// cacheDuration sets (in minutes) the duration for which the GET request's response will be cached for.
var cacheDuration = lookupEnvOrUseDefaultInt64("CACHE_DURATION", 15)

// rateLimit sets the maximum number of requests per second for a given IP address.
var rateLimit = lookupEnvOrUseDefaultFloat64("RATE_LIMIT", 1.0)

// recaptchaSecret is used for Google Recaptcha protection in POST requests.
// Use 6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe for testing
var recaptchaSecret = lookupEnvOrUseDefault("RECAPTCHA_SECRET", "6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe")
