package provider

import "time"

#Manifest: close({
	version:     "provider/v1"
	name:        string & =~"^[a-z][a-z0-9._-]*$"
	description: string & !=""
	command: [string & !="", ...string & !=""]
	actions: [=~"^[a-z][a-z0-9._-]*$"]: #Action
	requires?: #Requirements
	defaults?: #Defaults
})

#Action: close({
	description: string & !=""
	argv?: [...string]
	env?: [=~"^[A-Za-z_][A-Za-z0-9_]*$"]: string
})

#Requirements: close({
	commands?: [...string & !=""]
	environment?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$"]
	paths?: [...string & !=""]
})

#Defaults: close({
	timeout?:  time.Duration
	priority?: int
})

#Request: close({
	version:    "provider/v1"
	kind:       "request"
	requestId:  string & !=""
	capability: string & !=""
	operation?: string
	context?: [string]: _
	input?: _
})

#Event: close({
	version:   "provider/v1"
	kind:      "event"
	requestId: string & !=""
	event:     string & !=""
	message?:  string
	data?:     _
})

#Result: close({
	version:   "provider/v1"
	kind:      "result"
	requestId: string & !=""
	status:    "ok" | "declined" | "error"
	output?:   _
	message?:  string
	metadata?: [string]: string
})
