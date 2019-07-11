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

// port used to listen for connections.
var listenPort = lookupEnvOrUseDefaultInt64("PORT", 1323)

// cacheDuration sets (in minutes) the duration for which the GET request's response will be cached for.
var cacheDuration = lookupEnvOrUseDefaultInt64("CACHE_DURATION", 15)

// behindProxy sets whether the application sits behind a proxy. Valid values are 0 (No) or 1 (Yes).
var behindProxy = lookupEnvOrUseDefaultInt("BEHIND_PROXY", 0)

// rateLimit sets the maximum number of requests per second for a given IP address.
var rateLimit = lookupEnvOrUseDefaultFloat64("RATE_LIMIT", 4.0)

// Both gmailAccount and gmailPassword must be set to send account activation emails.
// If not set, accounts are automatically activated.
// serverHostUrl is the url root that the lemma chain server runs on. Activation links will use
// this url.
// website will be the website url that the activation link will redirect to.
var (
	website       = lookupEnvOrUseDefault("WEBSITE_URL", "")
	serverHostUrl = lookupEnvOrUseDefault("HOST_URL", "")
	gmailAccount  = lookupEnvOrUseDefault("GMAIL_ACCOUNT", "")
	gmailPassword = lookupEnvOrUseDefault("GMAIL_PASSWORD", "")
)

// recaptchaSecret is used for Google Recaptcha protection in POST requests.
// Use 6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe for testing
var recaptchaSecret = lookupEnvOrUseDefault("RECAPTCHA_SECRET", "6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe")
