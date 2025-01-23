# status-keycard-go

This library provides a higher level Keycard API for Status app. It is currently only used in [status-desktop](https://github.com/status-im/status-desktop/).

There are 2 types of API provided.

## Flow API 

Each keycard command is executed in a single _flow_. A flow roughly looks like this: 
   1. List available readers
   2. Look for a keycard
   3. Set up a connection
   4. Execute the command
   5. Close the connection

If client interaction is required at any stage (e.g. insert a card, input a PIN), the flow is "paused" and signals to the client. The client should manually continue the flow when the required action was performed. This basically drives the UI right from `status-keycard-go` library.  

> [!NOTE]
> status-desktop doesn't use this API anymore. Consider switching to Session API. 

## Session API

The main problem with Flow API is that it does not signal certain changes, e.g. "reader disconnected" and "card removed". Session API addresses this issue. 

The journey begins with `Start` endpoint. When the keycard service is started, it monitors all connected readers and cards. This allows to track the state of reader+card and notify the client on any change. As soon as a keycard is found, a "connection" (pair and open secure channel) is established automatically and will be reused until `Stop` is called or the keycard is removed. 

In the `Ready`/`Authorized` states client can execute wanted commands, each as a separate endpoint. 

Check out the detailed usage in ./api/README.md

