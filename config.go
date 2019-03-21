package main

// hashIDSalt is used to convert uids to hashids. Once set in production, do not modify
var hashIDSalt = lookupEnvOrUseDefault("HASHID_SALT", "ffb80dba55db4b7ab49cb83ed96eca29")

// maxDataPayload is used to set the maximum payload size in kB for a new node's custom data
var maxDataPayload = lookupEnvOrUseDefaultInt("MAX_PAYLOAD_KB", 12)

// stdQueryTimeout is used to set the maximum query duration (in ms) for potentially
// expensive GET requests.
const stdQueryTimeout = 300

// cacheDuration sets (in minutes) the duration for which the GET request's response will be cached for.
const cacheDuration = 15

// recaptchaSecret is used for Google Recaptcha protection in POST requests.
// Use 6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe for testing
var recaptchaSecret = lookupEnvOrUseDefault("RECAPTCHA_SECRET", "ffb80dba55db4b7ab49cb83ed96eca29")
