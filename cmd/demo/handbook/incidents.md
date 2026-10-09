# Incidents

An incident is declared when the collector is [down](/handbook) or when an
[alert](/alert) stays open for more than fifteen minutes. This page is what to do next.

## First five minutes

1. Say so in the channel: *who* is looking, and *what* they see.
2. Open the [alert](/alert/alt_000) that started it. Its device,
   [dev_27c38b](/device/dev_27c38b?minutes=15), shows the last fifteen minutes of events.
3. Compare the [Config](/config) page's diff with the last deploy. A configuration that was
   saved and never deployed is the most common cause.

## Severity

| Severity | When | Who is told |
| --- | --- | --- |
| `critical` | Nothing is being collected | the on-call and the site's owner |
| `warning` | A sink is slow or refusing | the on-call |
| `info` | A single device is quiet | nobody, until it is two |

Every critical alert is on the [Alerts](/alert) page. A device that keeps showing up
there is worth a look at its own page: [Devices](/device).

## Afterwards

- [ ] Write down what happened, in the [Audit log](/audit)'s own words
- [ ] Tell the site's owner, from the [Sites](/site) page
- [ ] Add what you learned to the [runbook](/handbook)

See also: [how the sinks work](/handbook/sinks) and the [glossary](/handbook/glossary).
