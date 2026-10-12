// What the collector gives a paging rule. The editor's text uses these names without
// declaring them: they come from the editor's Context, a folder the program embeds.

interface Signal {
  at: string;
  kind: "flush" | "connect" | "timeout";
  detail: string;
  ageMinutes: number;
}

interface Rule {
  name: string;
  matches(signal: Signal): boolean;
}

/** The rules the collector has loaded, in the order they are tried. */
declare const rules: Rule[];

/** Pages whoever is on call. "high" wakes them; "low" waits for the morning. */
declare function pageOnCall(signal: Signal, urgency: "low" | "high"): void;
