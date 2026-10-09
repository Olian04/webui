# Sinks

A **sink** is where the collector sends what it has gathered. Each one is configured in the
[Config](/config) page, under `sinks`, and each is its own service on the host.

| Kind | Writes to | Fails when |
| --- | --- | --- |
| `stdout` | the journal | never, but the journal can fill |
| `file` | a path on the host | the disk is full, or the path is not writable |
| `http` | a URL | the other end is down or slow |

The disk usage of a host is on the [System](/system) page, as a bar.

## Restarting a sink

```bash
systemctl restart collector-sink@file
```

Check that it came back before you leave: the queue depth on [System](/system) should
fall within a minute. If it does not, the sink is not the problem, and the
[incident page](/handbook/incidents) is where to go.

## Sampling

`sample` is the share of events that is kept, from 0 to 1. It is set per collector in the
[Config](/config) page:

```json
{ "collector": "eu-north-1", "sample": 0.25 }
```

A lower value eases a sink that cannot keep up, at the price of fewer events. The
[retention](/retention) of what is kept is set on its own page, and how much the collector
takes in is under [Ingest](/settings).

The words on this page are in the [glossary](/handbook/glossary). Back to the
[runbook](/handbook).
