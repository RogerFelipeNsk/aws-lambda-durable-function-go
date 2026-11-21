import type { TimelineEvent } from "../types";

export default function Timeline({ events }: { events?: TimelineEvent[] }) {
  if (!events?.length) return null;
  return (
    <ul className="timeline">
      {[...events].reverse().map((ev, i) => (
        <li key={`${ev.at}-${i}`}>
          <strong>{ev.type}</strong>
          {ev.message}
          <div>{new Date(ev.at).toLocaleString("pt-BR")}</div>
        </li>
      ))}
    </ul>
  );
}
