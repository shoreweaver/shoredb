---
title: Pub/Sub
section: Commands
order: 6
---

# Pub/Sub Commands

Channel-based publish/subscribe, backed by `pkg/pubsub`. Pub/Sub is **global** — it is not scoped to whichever logical database a connection has `SELECT`ed, matching real Redis's behavior.

## `PUBLISH`

```
PUBLISH channel message
```

Delivers `message` to every current subscriber of `channel`. Returns the number of subscribers the message was actually delivered to. Each subscriber has a small buffered delivery queue; a subscriber that's fallen behind is skipped for that message rather than blocking the publisher.

## `SUBSCRIBE`

```
SUBSCRIBE channel [channel ...]
```

Subscribes the current connection to one or more channels. For each channel, the server replies with a `subscribe` confirmation message containing the channel name and the connection's total subscription count. Messages published afterward arrive as three-element arrays: `["message", channel, payload]`.

## `UNSUBSCRIBE`

```
UNSUBSCRIBE [channel ...]
```

Unsubscribes from the given channels, or from all currently-subscribed channels if none are given. Replies with one `unsubscribe` confirmation per channel.

## Example session

```bash
# terminal 1
redis-cli -p 6379 SUBSCRIBE news

# terminal 2
redis-cli -p 6379 PUBLISH news "hello subscribers"
```

Terminal 1 receives the message as soon as it's published — delivery isn't scoped to the publisher's or subscriber's currently-selected database.
