<script lang="ts" module>
  // Every icon in the design is a 24×24 line glyph at stroke-width 2.75.
  // Paths are lifted from the design source so the set stays consistent
  // instead of drifting toward whatever each page invented.
  export const paths: Record<string, string> = {
    overview:
      "M3 3h7v9H3zM14 3h7v5h-7zM14 12h7v9h-7zM3 16h7v5H3z",
    hosts: "M4 12h5l2 4 3-9 2 5h4",
    certificates: "M4 10h16v10H4zM8 10V7a4 4 0 0 1 8 0v3",
    access: "M9 4.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6M17 11h4",
    logs: "M4 6h16M4 12h10M4 18h13",
    caddyfile: "m9 8-4 4 4 4M15 8l4 4-4 4",
    settings:
      "M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6 7 7M17 17l1.4 1.4M18.4 5.6 17 7M7 17l-1.4 1.4",
    shield: "M12 3 4 7v5c0 5 3.5 8 8 9 4.5-1 8-4 8-9V7Z",
    menu: "M4 7h16M4 12h16M4 17h16",
    theme: "M12 3a9 9 0 1 0 9 9 7 7 0 0 1-9-9Z",
    warn: "M12 8v5M12 17h.01M21 12a9 9 0 1 0-18 0 9 9 0 0 0 18 0",
    plus: "M12 5v14M5 12h14",
    trash: "M4 7h16M9 7V5h6v2M6 7l1 13h10l1-13",
    arrow: "M5 12h14m-6-6 6 6-6 6",
    refresh: "M20 11a8 8 0 1 0-.9 4.5M20 5v6h-6",
    download: "M12 4v11m0 0-4-4m4 4 4-4M5 20h14",
  };
</script>

<script lang="ts">
  let {
    name,
    size = 18,
    stroke = 2.75,
    fill = "none",
  }: { name: string; size?: number; stroke?: number; fill?: string } = $props();
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  {fill}
  stroke="currentColor"
  stroke-width={stroke}
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  <path d={paths[name]} />
</svg>
