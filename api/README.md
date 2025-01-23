> [!NOTE]  
> This guide is not comprehensive and relies on your wisdom and intelligence.  
> Only _session_ API is considered. For the _flow_ API please check out the source code.

# Description

This directory contains `*.http` request for each available endpoint in the Session API.

# Usage

Session API uses JSON-RPC protocol. All commands are available at `keycard` service. Here is an example:
```json
{
    "id": "1",
    "method": "keycard.Authorize",
    "params": [
        {
            "pin": "654321"
        }
    ]
}
```

There are 2 ways to access the API.

## HTTP

This way is easier to use for testing and debugging.

1. Run the server:
    ```shell
    go run ./cmd/status-keycard-server/main.go --address=localhost:12346
    ```

2. Connect to signals websocket at `ws://localhost:12346/signals`

3. Send requests to `http://localhost:12346/rpc`

## C bindings

This is the way to integrate `status-keycard-go` library, e.g. how `status-desktop` uses it.

To subscribe to signals, set a callback function with `setSignalEventCallback`.

For the RPC server, there are 2 methods provided: 
1. `InitializeRPC` - must be called once at the start of the application, before making any RPC calls.
2. `CallRPC` - call with a single JSON string argument according to the JSON-RPC protocol. Returns a single JSON string response.


# Setup

1. Connect to signals  
   For the session API, the only emitted signal is `status-changed`.   
   It provides current status of the session and information about connected keycard.
2. Call `Start`  
   From this moment, until `Stop` is called, the keycard service will take care of watching readers/cards and keeping a secure "connection" with a keycard.  
   Provide `StorageFilePath`, e.g. `~/pairings.json`. This file will be used to store pairing keys for all paired keycards.
3. If planning to execute any authorization-required commands, call `Authorize`
4. Monitor state of the session, execute any needed commands.
   NOTE: Some of the keycard commands can only be executed in `ready` or `authorized` state.
5. Call `Stop`

# Simulation

Because it is difficult (perhaps nearly impossible) to implement proper simulation of a keycard, 
this library provides a way to simulate certain errors, which is not simple/possible to achieve with hardware.

Check [`SimulateErrro`](#simulateerror) method for details

# API

## Signals

Signals follow the structure described here: https://github.com/keycard-tech/status-keycard-go/blob/b1e1f7f0bf534269a5c18fcd31649d2056b13e5b/signal/signals.go#L27-L31

The only signal type used in Session API is `status-changed`. For event structure, check out [Status](#status)

## Service endpoints

These endpoints are related to the `status-keycard-go` library itself:

## `Start`

Starts the monitoring of readers and cards.

The monitoring starts with _detect_ mode. 
In this mode it checks all connected readers for a smart cards. Monitoring supports events (like reader connection and card insertion) to happen even after calling `Start`.

As soon as a reader with a Keycard is found, the monitoring switches to _watch_ mode:
- Only the reader with the keycard is watched. If the keycard is removed, or the reader is disconnected, the monitoring goes back to _detect_ mode.
- Any new connected readers, or inserted smart cards on other readers, are ignored. 

[//]: # (TODO: Diagram)

## `Stop` 

Stops the monitoring.

## `SimulateError`

Marks that certain error should be simulated.

For the `simulated-not-a-keycard` error, `InstanceUID` argument must be provided. Only keycards with such `InstanceUID` will be treated as not keycards.
Other errors are applied no matter of the `InstanceUID` value.

`SimulateError` can also be called before `Start`, e.g. to simulate `simulated-no-pcsc` error, as this one can only happen during `Start` operation.

Use `SimulateError` method with one of the supported simulation errors: https://github.com/keycard-tech/status-keycard-go/blob/a3804cc8848a93a277895e508dd7c423f1f8338c/internal/keycard_context_v2_state.go#L55-L62

## `GetStatus`

Returns current status of the session.

In most scenarios `status-changed` signal should be used to get status. Yet in some debugging circumstances this method can be handy to get the latest status. 

## Status

Here is the structure of the status: https://github.com/keycard-tech/status-keycard-go/blob/a3804cc8848a93a277895e508dd7c423f1f8338c/internal/keycard_context_v2_state.go#L30-L35

The main field is `state`

### State

Check the source code for the list of possible states and their description.
https://github.com/keycard-tech/status-keycard-go/blob/a3804cc8848a93a277895e508dd7c423f1f8338c/internal/keycard_context_v2_state.go#L11-L73

## Commands

Apart from the service endpoints listed above, all other endpoints represent the actual [Keycard API](https://keycard.tech/docs/apdu).

Most of the commands have to be executed in `ready` or `authorized` states. Service will return a readable error if the keycard is not in the proper state for the command.

Please check out the Keycard documentation for more details.

## Examples

The examples are presented in a "you'll get it" form.  
`<-` represents a reception of `status-changed` signal.

### Initialize a new Keycard

```go
Start("~/pairings.json")
<- "waiting-for-reader"
// connect a reader
<- "waiting-for-card"
// insert a keycard
<- "connecting-card"
<- "empty-keycard", AppInfo: { InstanceUID: "abcd" }, AppStatus: null
Initialize(pin: "123456", puk: "123456123456")
<- "ready", Appinfo: ..., AppStatus: { remainingAttemptsPIN: 3, remainingAttemptsPUK: 5, ... }
Authorize(pin: "123456")
<- "autorized", AppInfo: ..., AppStatus ...
ExportLoginKeys()
```

### Unblock a Keycard

```go
Start("~/pairings.json")
<- "waiting-for-reader"
// connect a reader with a keycard
<- "connecting-card"
<- "blocked-pin", AppInfo: { InstanceUID: "abcd" }, AppStatus: { remainingAttemptsPIN: 0, remainingAttemptsPUK: 5, ... }
UnblockPIN(puk: "123456123456")
<- "authorized", AppInfo: ..., AppStatus: { remainingAttemptsPIN: 3, remainingAttemptsPUK: 5, ... }
```

### Factory reset a completely blocked Keycard

```go
Start("~/pairings.json")
<- "waiting-for-reader"
// connect a reader with a keycard
<- "connecting-card"
<- "blocked-puk", AppInfo: { InstanceUID: "abcd" }, AppStatus: { remainingAttemptsPIN: 0, remainingAttemptsPUK: 0, ... }
FactoryReset()
<- "factory-resetting"
<- "empty-keycard"
```

# Implementation decisions

1. Monitoring detect mode utilizes [`\\?PnP?\Notification`](https://blog.apdu.fr/posts/2024/08/improved-scardgetstatuschange-for-pnpnotification-special-reader/) feature to detect new connected readers without any CPU load.
2. Monitoring watch mode could use a blocking call to `GetStatusChange`, but this did not work on Linux (Ubuntu), although worked on MacOS.  
So instead there is a loop that checks the state of the reader each 500ms.
3. JSON-RPC was chosen for 2 reasons:
    - to expose API to HTTP for testing/debugging
    - to simplify the adding new methods to the API
gRPC was also considered, but this would require more work on `status-desktop`.
