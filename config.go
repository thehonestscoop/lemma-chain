package main

// hashIDSalt is used to convert uids to hashids. Once set in production,
// do not modify
const hashIDSalt = "ffb80dba55db4b7ab49cb83ed96eca29"

const maxDataPayload = 12 // 12kB

const stdQueryTimeout = 300 // 300ms

const cacheDuration = 5 // 5 minutes
