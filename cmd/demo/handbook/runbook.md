# Collector handbook

What to do when the collector **misbehaves**. Anything not covered here goes to the
[on-call channel](https://example.com/oncall).

The handbook has four pages: this runbook, [what to do in an incident](/handbook/incidents),
[how the sinks work](/handbook/sinks), and a [glossary](/handbook/glossary).

## Is it down?

1. Open [System](/system) and read the *health badge*.
2. Check the queue depth. Above 5 000 the sinks are not keeping up.
3. Look at the [Audit log](/audit) for a change in the last hour:
   - a configuration saved by someone else
   - a sink that was restarted without a reason

| Health | Meaning | First step |
| --- | --- | --- |
| `healthy` | Everything is flowing | none |
| `degraded` | A sink is slow or refusing | [restart the sink](/handbook/sinks) |
| `down` | Nothing is being collected | page the on-call, then [open an incident](/handbook/incidents) |

---
## Restarting a sink

```bash
systemctl restart collector-sink@file
journalctl -u collector-sink@file --since '5 min ago'
```

The configuration it reads is on the [Config](/config) page:

```json
{ "sample": 0.25, "sinks": [{ "kind": "stdout" }] }
```

### Who to call

Start with the person on call, and go on to the owner of the site only when the queue
is still growing after *ten minutes*. The sites and their owners are on the
[Sites](/site) page, and [Stockholm](/site/Stockholm) is the one that pages most.

#### Contacts

Numbers are kept in `/etc/collector/oncall.yml` on every host.

## Checklist

- [x] Page acknowledged
- [ ] Cause found
- [ ] ~~Rollback~~ not needed

> Do not edit the configuration during an incident without telling the channel.

<script>alert('raw HTML is never rendered')</script>
