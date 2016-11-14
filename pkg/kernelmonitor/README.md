# Kernel Monitor

*Kernel Monitor* is a problem daemon in node problem detector. It monitors kernel log
and detects known kernel issues following predefined rules.

The Kernel Monitor matches kernel issues according to a set of predefined rule list in
[`config/kernel-monitor.json`](https://github.com/kubernetes/node-problem-detector/blob/master/config/kernel-monitor.json).
The rule list is extensible.

## Limitations

* Kernel Monitor only reads logs from `/dev/kmsg`. This device is only
  available on Linux with a kernel of at least version 3.5.
  For any non-Linux OS or poorly configured kernels, this may not work.
* Kernel Monitor may have inaccurate timestamps. This is due to the design of
  the Kernel Ring buffer, which is susceptible to suspend/hibernate throwing
  off message timestamps. Use of suspend/hibernate alongside Kernel Monitor is
  not supported.

## Add New NodeConditions

To support new node conditions, you can extend the `conditions` field in
`config/kernel-monitor.json` with new condition definition:

```json
{
  "type": "NodeConditionType",
  "reason": "CamelCaseDefaultNodeConditionReason",
  "message": "arbitrary default node condition message"
}
```

## Detect New Problems

To detect new problems, you can extend the `rules` field in `config/kernel-monitor.json`
with new rule definition:

```json
{
  "type": "temporary/permanent",
  "condition": "NodeConditionOfPermanentIssue",
  "reason": "CamelCaseShortReason",
  "message": "regexp matching the issue in the kernel log"
}
```
